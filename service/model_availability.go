package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_availability_setting"
)

const (
	availabilityRequestTimeout = 3 * time.Second
	availabilityCacheDuration  = 30 * time.Second
	availabilityMaxBodyBytes   = 64 * 1024
)

type AvailabilityState struct {
	Status string `json:"status"`
}

type GroupAvailability struct {
	Status string                       `json:"status"`
	Models map[string]AvailabilityState `json:"models,omitempty"`
}

type AvailabilityResult struct {
	Groups map[string]GroupAvailability `json:"groups"`
}

type upstreamAvailability struct {
	Status     string                       `json:"status"`
	ObservedAt int64                        `json:"observed_at"`
	TTLSeconds int64                        `json:"ttl_seconds"`
	Models     map[string]AvailabilityState `json:"models,omitempty"`
}

type cachedAvailability struct {
	value     upstreamAvailability
	expiresAt time.Time
}

var availabilityCache = struct {
	sync.Mutex
	entries map[[32]byte]cachedAvailability
}{entries: make(map[[32]byte]cachedAvailability)}

func GetModelAvailability(ctx context.Context) AvailabilityResult {
	setting := model_availability_setting.GetSetting()
	if !setting.Enabled {
		return AvailabilityResult{Groups: map[string]GroupAvailability{}}
	}
	sources, err := model_availability_setting.ParseSources(setting.Sources)
	if err != nil || len(sources) == 0 {
		return AvailabilityResult{Groups: map[string]GroupAvailability{}}
	}
	return collectModelAvailability(ctx, sources, GetHttpClient(), time.Now())
}

func collectModelAvailability(ctx context.Context, sources []model_availability_setting.Source, client *http.Client, now time.Time) AvailabilityResult {
	type sourceResult struct {
		group string
		value *upstreamAvailability
	}
	results := make(chan sourceResult, len(sources))
	for _, source := range sources {
		source := source
		go func() {
			value, err := fetchAvailability(ctx, source, client, now)
			if err != nil {
				results <- sourceResult{group: source.Group}
				return
			}
			results <- sourceResult{group: source.Group, value: &value}
		}()
	}

	configured := make(map[string]int)
	responses := make(map[string][]upstreamAvailability)
	for _, source := range sources {
		configured[source.Group]++
	}
	for range sources {
		result := <-results
		if result.value != nil {
			responses[result.group] = append(responses[result.group], *result.value)
		}
	}

	groups := make(map[string]GroupAvailability)
	for group, expected := range configured {
		values := responses[group]
		status, ok := aggregateAvailability(values, expected, "")
		if !ok {
			continue
		}
		models := make(map[string]AvailabilityState)
		modelNames := make(map[string]struct{})
		for _, value := range values {
			for model := range value.Models {
				modelNames[model] = struct{}{}
			}
		}
		for model := range modelNames {
			if modelStatus, modelOK := aggregateAvailability(values, expected, model); modelOK && modelStatus != status {
				models[model] = AvailabilityState{Status: modelStatus}
			}
		}
		groups[group] = GroupAvailability{Status: status, Models: models}
	}
	return AvailabilityResult{Groups: groups}
}

func aggregateAvailability(values []upstreamAvailability, expected int, model string) (string, bool) {
	allUnavailable := len(values) == expected
	allMaintenance := allUnavailable
	for _, value := range values {
		status := value.Status
		if override, ok := value.Models[model]; model != "" && ok {
			status = override.Status
		}
		if status == "available" {
			return "available", true
		}
		if status != "unavailable" && status != "maintenance" {
			allUnavailable = false
		}
		if status != "maintenance" {
			allMaintenance = false
		}
	}
	if allMaintenance {
		return "maintenance", true
	}
	if allUnavailable {
		return "unavailable", true
	}
	return "", false
}

func fetchAvailability(ctx context.Context, source model_availability_setting.Source, client *http.Client, now time.Time) (upstreamAvailability, error) {
	cacheKey := sha256.Sum256([]byte(source.URL + "\x00" + source.Token))
	availabilityCache.Lock()
	entry, found := availabilityCache.entries[cacheKey]
	availabilityCache.Unlock()
	if found && now.Before(entry.expiresAt) {
		return entry.value, nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, availabilityRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, source.URL, nil)
	if err != nil {
		return upstreamAvailability{}, err
	}
	request.Header.Set("Accept", "application/json")
	if source.Token != "" {
		request.Header.Set("Authorization", "Bearer "+source.Token)
	}
	response, err := client.Do(request)
	if err != nil {
		return upstreamAvailability{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return upstreamAvailability{}, fmt.Errorf("availability source returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, availabilityMaxBodyBytes+1))
	if err != nil || len(body) > availabilityMaxBodyBytes {
		return upstreamAvailability{}, fmt.Errorf("invalid availability response size")
	}
	var value upstreamAvailability
	if err := common.Unmarshal(body, &value); err != nil {
		return upstreamAvailability{}, err
	}
	if !validAvailabilityStatus(value.Status) || value.TTLSeconds < 1 || value.TTLSeconds > 300 {
		return upstreamAvailability{}, fmt.Errorf("invalid availability response")
	}
	for _, model := range value.Models {
		if !validAvailabilityStatus(model.Status) {
			return upstreamAvailability{}, fmt.Errorf("invalid model availability response")
		}
	}
	observedAt := time.Unix(value.ObservedAt, 0)
	if observedAt.After(now.Add(30*time.Second)) || !now.Before(observedAt.Add(time.Duration(value.TTLSeconds)*time.Second)) {
		return upstreamAvailability{}, fmt.Errorf("stale availability response")
	}
	expiresAt := now.Add(availabilityCacheDuration)
	if upstreamExpiry := observedAt.Add(time.Duration(value.TTLSeconds) * time.Second); upstreamExpiry.Before(expiresAt) {
		expiresAt = upstreamExpiry
	}
	availabilityCache.Lock()
	availabilityCache.entries[cacheKey] = cachedAvailability{value: value, expiresAt: expiresAt}
	availabilityCache.Unlock()
	return value, nil
}

func validAvailabilityStatus(status string) bool {
	switch status {
	case "available", "unavailable", "maintenance", "unknown":
		return true
	default:
		return false
	}
}

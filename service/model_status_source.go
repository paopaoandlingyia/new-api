package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_status_setting"
)

const (
	modelStatusSourcePollInterval = 10 * time.Second
	modelStatusSourceTimeout      = 2 * time.Second
	modelStatusSourceMaxStaleness = 30 * time.Second
	modelStatusSourceMaxBodyBytes = 64 << 10
)

type ModelStatusSourceInput struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	APIKey      string            `json:"api_key"`
	ClearAPIKey bool              `json:"clear_api_key"`
	Enabled     bool              `json:"enabled"`
	Mappings    map[string]string `json:"mappings"`
}

type ModelStatusSourceView struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	URL           string            `json:"url"`
	HasAPIKey     bool              `json:"has_api_key"`
	Enabled       bool              `json:"enabled"`
	Mappings      map[string]string `json:"mappings"`
	LastSuccessAt int64             `json:"last_success_at,omitempty"`
	LastError     string            `json:"last_error,omitempty"`
}

type modelStatusSourceResponse struct {
	GeneratedAt  int64           `json:"generated_at"`
	Availability map[string]bool `json:"availability"`
}

type modelStatusSourceObservation struct {
	Availability  map[string]bool
	GeneratedAt   int64
	ReceivedAt    time.Time
	LastAttemptAt int64
	LastError     string
}

var (
	modelStatusSourceRuntime = struct {
		sync.RWMutex
		Observations map[string]modelStatusSourceObservation
	}{Observations: make(map[string]modelStatusSourceObservation)}
	modelStatusSourceUpdateMutex sync.Mutex
	modelStatusSourcePollWake    = make(chan struct{}, 1)
	ErrInvalidModelStatusSource  = errors.New("invalid model status source")
)

func StartModelStatusSourcePoller() error {
	if _, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources); err != nil {
		return err
	}
	go func() {
		client := &http.Client{Timeout: modelStatusSourceTimeout}
		ticker := time.NewTicker(modelStatusSourcePollInterval)
		defer ticker.Stop()
		for {
			pollModelStatusSources(client)
			select {
			case <-ticker.C:
			case <-modelStatusSourcePollWake:
			}
		}
	}()
	return nil
}

func pollModelStatusSources(client *http.Client) {
	sources, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources)
	if err != nil {
		common.SysError("model status sources are invalid: " + err.Error())
		return
	}
	configured := make(map[string]struct{}, len(sources))
	var waitGroup sync.WaitGroup
	for _, source := range sources {
		configured[source.ID] = struct{}{}
		if !source.Enabled {
			continue
		}
		waitGroup.Add(1)
		go func(source model_status_setting.Source) {
			defer waitGroup.Done()
			response, fetchErr := fetchModelStatusSource(context.Background(), client, source)
			now := time.Now()
			modelStatusSourceRuntime.Lock()
			observation := modelStatusSourceRuntime.Observations[source.ID]
			wasFailing := observation.LastError != ""
			observation.LastAttemptAt = now.Unix()
			if fetchErr != nil {
				observation.LastError = fetchErr.Error()
				modelStatusSourceRuntime.Observations[source.ID] = observation
				modelStatusSourceRuntime.Unlock()
				if !wasFailing {
					common.SysError(fmt.Sprintf("model status source %q fetch failed: %v", source.Name, fetchErr))
				}
				return
			}
			observation.Availability = response.Availability
			observation.GeneratedAt = response.GeneratedAt
			observation.ReceivedAt = now
			observation.LastError = ""
			modelStatusSourceRuntime.Observations[source.ID] = observation
			modelStatusSourceRuntime.Unlock()
			if wasFailing {
				common.SysLog(fmt.Sprintf("model status source %q connection recovered", source.Name))
			}
		}(source)
	}
	waitGroup.Wait()

	modelStatusSourceRuntime.Lock()
	for sourceID := range modelStatusSourceRuntime.Observations {
		if _, exists := configured[sourceID]; !exists {
			delete(modelStatusSourceRuntime.Observations, sourceID)
		}
	}
	modelStatusSourceRuntime.Unlock()
}

func fetchModelStatusSource(ctx context.Context, client *http.Client, source model_status_setting.Source) (modelStatusSourceResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return modelStatusSourceResponse{}, fmt.Errorf("create request: %w", err)
	}
	if source.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+source.APIKey)
	}
	response, err := client.Do(request)
	if err != nil {
		return modelStatusSourceResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return modelStatusSourceResponse{}, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	var payload modelStatusSourceResponse
	if err := common.DecodeJson(io.LimitReader(response.Body, modelStatusSourceMaxBodyBytes), &payload); err != nil {
		return modelStatusSourceResponse{}, fmt.Errorf("decode response: %w", err)
	}
	if payload.GeneratedAt < 1 {
		return modelStatusSourceResponse{}, fmt.Errorf("response generated_at is invalid")
	}
	for _, remoteKey := range source.Mappings {
		if _, exists := payload.Availability[remoteKey]; !exists {
			return modelStatusSourceResponse{}, fmt.Errorf("response omits mapped key %q", remoteKey)
		}
	}
	return payload, nil
}

func GetModelStatusSources() ([]ModelStatusSourceView, error) {
	sources, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources)
	if err != nil {
		return nil, err
	}
	modelStatusSourceRuntime.RLock()
	defer modelStatusSourceRuntime.RUnlock()
	views := make([]ModelStatusSourceView, 0, len(sources))
	for _, source := range sources {
		observation := modelStatusSourceRuntime.Observations[source.ID]
		lastSuccessAt := int64(0)
		if !observation.ReceivedAt.IsZero() {
			lastSuccessAt = observation.ReceivedAt.Unix()
		}
		views = append(views, ModelStatusSourceView{
			ID:            source.ID,
			Name:          source.Name,
			URL:           source.URL,
			HasAPIKey:     source.APIKey != "",
			Enabled:       source.Enabled,
			Mappings:      source.Mappings,
			LastSuccessAt: lastSuccessAt,
			LastError:     observation.LastError,
		})
	}
	return views, nil
}

func CreateModelStatusSource(input ModelStatusSourceInput) error {
	modelStatusSourceUpdateMutex.Lock()
	defer modelStatusSourceUpdateMutex.Unlock()
	sources, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources)
	if err != nil {
		return err
	}
	source, err := normalizeModelStatusSourceInput(common.GetUUID(), input, input.APIKey)
	if err != nil {
		return err
	}
	sources = append(sources, source)
	return persistModelStatusSources(sources)
}

func UpdateModelStatusSource(sourceID string, input ModelStatusSourceInput) error {
	modelStatusSourceUpdateMutex.Lock()
	defer modelStatusSourceUpdateMutex.Unlock()
	sources, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources)
	if err != nil {
		return err
	}
	for index, source := range sources {
		if source.ID != sourceID {
			continue
		}
		apiKey := source.APIKey
		if input.ClearAPIKey {
			apiKey = ""
		} else if input.APIKey != "" {
			apiKey = input.APIKey
		}
		updated, normalizeErr := normalizeModelStatusSourceInput(sourceID, input, apiKey)
		if normalizeErr != nil {
			return normalizeErr
		}
		sources[index] = updated
		return persistModelStatusSources(sources)
	}
	return fmt.Errorf("%w: source %q does not exist", ErrInvalidModelStatusSource, sourceID)
}

func DeleteModelStatusSource(sourceID string) error {
	modelStatusSourceUpdateMutex.Lock()
	defer modelStatusSourceUpdateMutex.Unlock()
	sources, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources)
	if err != nil {
		return err
	}
	for index, source := range sources {
		if source.ID != sourceID {
			continue
		}
		sources = append(sources[:index], sources[index+1:]...)
		return persistModelStatusSources(sources)
	}
	return fmt.Errorf("%w: source %q does not exist", ErrInvalidModelStatusSource, sourceID)
}

func normalizeModelStatusSourceInput(sourceID string, input ModelStatusSourceInput, apiKey string) (model_status_setting.Source, error) {
	mappings := make(map[string]string, len(input.Mappings))
	for localGroup, remoteKey := range input.Mappings {
		mappings[strings.TrimSpace(localGroup)] = strings.TrimSpace(remoteKey)
	}
	source := model_status_setting.Source{
		ID:       sourceID,
		Name:     strings.TrimSpace(input.Name),
		URL:      strings.TrimSpace(input.URL),
		APIKey:   strings.TrimSpace(apiKey),
		Enabled:  input.Enabled,
		Mappings: mappings,
	}
	payload, err := common.Marshal([]model_status_setting.Source{source})
	if err != nil {
		return model_status_setting.Source{}, fmt.Errorf("%w: marshal source: %v", ErrInvalidModelStatusSource, err)
	}
	if _, err := model_status_setting.ParseSources(string(payload)); err != nil {
		return model_status_setting.Source{}, fmt.Errorf("%w: %v", ErrInvalidModelStatusSource, err)
	}
	managedGroups := managedGroupDescriptions(model.GetPricing())
	for localGroup := range source.Mappings {
		if _, exists := managedGroups[localGroup]; !exists {
			return model_status_setting.Source{}, fmt.Errorf("%w: group %q is not present in the model catalog", ErrInvalidModelStatusSource, localGroup)
		}
	}
	return source, nil
}

func persistModelStatusSources(sources []model_status_setting.Source) error {
	payload, err := common.Marshal(sources)
	if err != nil {
		return fmt.Errorf("marshal model status sources: %w", err)
	}
	if err := model.UpdateOption(model_status_setting.SourcesOptionKey, string(payload)); err != nil {
		return err
	}
	select {
	case modelStatusSourcePollWake <- struct{}{}:
	default:
	}
	return nil
}

func automaticGroupStatus(groupName string, now time.Time) (string, int64, bool) {
	sources, err := model_status_setting.ParseSources(model_status_setting.GetSetting().Sources)
	if err != nil {
		return "", 0, false
	}
	modelStatusSourceRuntime.RLock()
	defer modelStatusSourceRuntime.RUnlock()
	automated := false
	available := false
	updatedAt := int64(0)
	for _, source := range sources {
		remoteKey, mapped := source.Mappings[groupName]
		if !source.Enabled || !mapped {
			continue
		}
		automated = true
		observation, observed := modelStatusSourceRuntime.Observations[source.ID]
		if !observed || observation.ReceivedAt.IsZero() || now.Sub(observation.ReceivedAt) > modelStatusSourceMaxStaleness {
			continue
		}
		updatedAt = max(updatedAt, observation.GeneratedAt)
		available = available || observation.Availability[remoteKey]
	}
	if !automated {
		return "", 0, false
	}
	// A transient network failure does not prove the upstream lost all accounts,
	// so recent observations remain valid for one short grace window. Fetch
	// failures are logged per source; after the grace window that source no longer
	// contributes availability and the OR aggregation fails closed to maintenance.
	if available {
		return model_status_setting.StatusAvailable, updatedAt, true
	}
	return model_status_setting.StatusMaintenance, updatedAt, true
}

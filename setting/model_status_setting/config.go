package model_status_setting

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const GroupsOptionKey = "model_status_setting.groups"
const SourcesOptionKey = "model_status_setting.sources_secret"

const (
	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
	StatusMaintenance = "maintenance"
)

type Entry struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

type Setting struct {
	Groups  string `json:"groups"`
	Sources string `json:"sources_secret"`
}

type Source struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	APIKey   string            `json:"api_key,omitempty"`
	Enabled  bool              `json:"enabled"`
	Mappings map[string]string `json:"mappings"`
}

var modelStatusSetting = Setting{Groups: "{}", Sources: "[]"}

func init() {
	config.GlobalConfig.Register("model_status_setting", &modelStatusSetting)
}

func GetSetting() Setting {
	return modelStatusSetting
}

func ParseGroups(value string) (map[string]Entry, error) {
	groups := make(map[string]Entry)
	if err := common.UnmarshalJsonStr(value, &groups); err != nil {
		return nil, fmt.Errorf("invalid group status entries: %w", err)
	}
	if len(groups) > 500 {
		return nil, fmt.Errorf("group status entries cannot exceed 500 entries")
	}
	for groupName, entry := range groups {
		if groupName != strings.TrimSpace(groupName) || groupName == "" || len(groupName) > 128 {
			return nil, fmt.Errorf("group status contains an invalid group name")
		}
		if !IsValidStatus(entry.Status) {
			return nil, fmt.Errorf("group status for %q is invalid", groupName)
		}
		if utf8.RuneCountInString(entry.Message) > 500 {
			return nil, fmt.Errorf("group status message for %q exceeds 500 characters", groupName)
		}
		if entry.UpdatedAt < 1 {
			return nil, fmt.Errorf("group status update time for %q is invalid", groupName)
		}
	}
	return groups, nil
}

func ValidateGroups(value string) error {
	_, err := ParseGroups(value)
	return err
}

func ParseSources(value string) ([]Source, error) {
	sources := make([]Source, 0)
	if err := common.UnmarshalJsonStr(value, &sources); err != nil {
		return nil, fmt.Errorf("invalid model status sources: %w", err)
	}
	if len(sources) > 50 {
		return nil, fmt.Errorf("model status sources cannot exceed 50 entries")
	}
	ids := make(map[string]struct{}, len(sources))
	for i, source := range sources {
		if source.ID != strings.TrimSpace(source.ID) || source.ID == "" || len(source.ID) > 64 {
			return nil, fmt.Errorf("model status source %d has an invalid id", i+1)
		}
		if _, duplicate := ids[source.ID]; duplicate {
			return nil, fmt.Errorf("model status source id %q is duplicated", source.ID)
		}
		ids[source.ID] = struct{}{}
		if source.Name != strings.TrimSpace(source.Name) || source.Name == "" || utf8.RuneCountInString(source.Name) > 100 {
			return nil, fmt.Errorf("model status source %q has an invalid name", source.ID)
		}
		if len(source.URL) > 2048 {
			return nil, fmt.Errorf("model status source %q URL is too long", source.ID)
		}
		parsedURL, err := url.Parse(source.URL)
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return nil, fmt.Errorf("model status source %q URL must be absolute HTTP(S)", source.ID)
		}
		if len(source.APIKey) > 4096 {
			return nil, fmt.Errorf("model status source %q API key is too long", source.ID)
		}
		if len(source.Mappings) == 0 || len(source.Mappings) > 500 {
			return nil, fmt.Errorf("model status source %q must contain 1 to 500 mappings", source.ID)
		}
		for localGroup, remoteKey := range source.Mappings {
			if localGroup != strings.TrimSpace(localGroup) || localGroup == "" || len(localGroup) > 128 ||
				remoteKey != strings.TrimSpace(remoteKey) || remoteKey == "" || len(remoteKey) > 128 {
				return nil, fmt.Errorf("model status source %q contains an invalid mapping", source.ID)
			}
		}
	}
	return sources, nil
}

func ValidateSources(value string) error {
	_, err := ParseSources(value)
	return err
}

func IsValidStatus(status string) bool {
	switch status {
	case StatusAvailable, StatusUnavailable, StatusMaintenance:
		return true
	default:
		return false
	}
}

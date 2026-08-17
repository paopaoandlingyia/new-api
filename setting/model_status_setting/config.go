package model_status_setting

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const ItemsOptionKey = "model_status_setting.items"

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
	Items string `json:"items"`
}

var modelStatusSetting = Setting{Items: "{}"}

func init() {
	config.GlobalConfig.Register("model_status_setting", &modelStatusSetting)
}

func GetSetting() Setting {
	return modelStatusSetting
}

func ParseItems(value string) (map[string]Entry, error) {
	items := make(map[string]Entry)
	if err := common.UnmarshalJsonStr(value, &items); err != nil {
		return nil, fmt.Errorf("invalid model status items: %w", err)
	}
	if len(items) > 2000 {
		return nil, fmt.Errorf("model status items cannot exceed 2000 entries")
	}
	for modelName, entry := range items {
		if modelName != strings.TrimSpace(modelName) || modelName == "" || len(modelName) > 256 {
			return nil, fmt.Errorf("model status contains an invalid model name")
		}
		if !IsValidStatus(entry.Status) {
			return nil, fmt.Errorf("model status for %q is invalid", modelName)
		}
		if utf8.RuneCountInString(entry.Message) > 500 {
			return nil, fmt.Errorf("model status message for %q exceeds 500 characters", modelName)
		}
		if entry.UpdatedAt < 1 {
			return nil, fmt.Errorf("model status update time for %q is invalid", modelName)
		}
	}
	return items, nil
}

func ValidateItems(value string) error {
	_, err := ParseItems(value)
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

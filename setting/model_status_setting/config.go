package model_status_setting

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const GroupsOptionKey = "model_status_setting.groups"

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
	Groups string `json:"groups"`
}

var modelStatusSetting = Setting{Groups: "{}"}

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

func IsValidStatus(status string) bool {
	switch status {
	case StatusAvailable, StatusUnavailable, StatusMaintenance:
		return true
	default:
		return false
	}
}

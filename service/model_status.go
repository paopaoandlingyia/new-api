package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_status_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type GroupStatus struct {
	GroupName   string   `json:"group_name"`
	Description string   `json:"description,omitempty"`
	Enabled     bool     `json:"enabled"`
	Status      string   `json:"status"`
	Message     string   `json:"message,omitempty"`
	UpdatedAt   int64    `json:"updated_at,omitempty"`
	Models      []string `json:"models"`
	Automated   bool     `json:"automated,omitempty"`
}

type GroupAvailabilitySummary struct {
	UpdatedAt int64             `json:"updated_at"`
	Groups    map[string]string `json:"groups"`
}

type GroupStatusUpdate struct {
	GroupName string `json:"group_name"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

var groupStatusUpdateMutex sync.Mutex

var ErrInvalidGroupStatusUpdate = errors.New("invalid group status update")

func GetPublishedGroupStatuses(usableGroups map[string]string) ([]GroupStatus, error) {
	groups, err := model_status_setting.ParseGroups(model_status_setting.GetSetting().Groups)
	if err != nil {
		return nil, err
	}
	return mergeGroupStatuses(model.GetPricing(), usableGroups, groups, false), nil
}

func GetManagedGroupStatuses() ([]GroupStatus, error) {
	groups, err := model_status_setting.ParseGroups(model_status_setting.GetSetting().Groups)
	if err != nil {
		return nil, err
	}
	pricing := model.GetPricing()
	return mergeGroupStatuses(pricing, managedGroupDescriptions(pricing), groups, true), nil
}

func GetPublishedGroupAvailability(usableGroups map[string]string) (GroupAvailabilitySummary, error) {
	statuses, err := GetPublishedGroupStatuses(usableGroups)
	if err != nil {
		return GroupAvailabilitySummary{}, err
	}
	return summarizeGroupAvailability(statuses), nil
}

func summarizeGroupAvailability(statuses []GroupStatus) GroupAvailabilitySummary {
	summary := GroupAvailabilitySummary{Groups: make(map[string]string, len(statuses))}
	for _, status := range statuses {
		summary.Groups[status.GroupName] = status.Status
		if status.UpdatedAt > summary.UpdatedAt {
			summary.UpdatedAt = status.UpdatedAt
		}
	}
	return summary
}

func UpdateGroupStatus(update GroupStatusUpdate) error {
	update.GroupName = strings.TrimSpace(update.GroupName)
	update.Message = strings.TrimSpace(update.Message)
	if update.GroupName == "" || len(update.GroupName) > 128 || update.GroupName == "auto" || update.GroupName == "all" {
		return fmt.Errorf("%w: invalid group name", ErrInvalidGroupStatusUpdate)
	}
	if utf8.RuneCountInString(update.Message) > 500 {
		return fmt.Errorf("%w: group status message cannot exceed 500 characters", ErrInvalidGroupStatusUpdate)
	}
	if update.Enabled && !model_status_setting.IsValidStatus(update.Status) {
		return fmt.Errorf("%w: invalid group status", ErrInvalidGroupStatusUpdate)
	}

	pricing := model.GetPricing()
	managed := mergeGroupStatuses(pricing, managedGroupDescriptions(pricing), nil, true)
	groupExists := false
	for _, group := range managed {
		if group.GroupName == update.GroupName {
			groupExists = true
			break
		}
	}
	if !groupExists {
		return fmt.Errorf("%w: group %q is not present in the model catalog", ErrInvalidGroupStatusUpdate, update.GroupName)
	}

	groupStatusUpdateMutex.Lock()
	defer groupStatusUpdateMutex.Unlock()
	groups, err := model_status_setting.ParseGroups(model_status_setting.GetSetting().Groups)
	if err != nil {
		return err
	}
	if update.Enabled {
		groups[update.GroupName] = model_status_setting.Entry{
			Status:    update.Status,
			Message:   update.Message,
			UpdatedAt: time.Now().Unix(),
		}
	} else {
		delete(groups, update.GroupName)
	}
	payload, err := common.Marshal(groups)
	if err != nil {
		return fmt.Errorf("marshal group status entries: %w", err)
	}
	return model.UpdateOption(model_status_setting.GroupsOptionKey, string(payload))
}

func managedGroupDescriptions(pricing []model.Pricing) map[string]string {
	descriptions := setting.GetUserUsableGroupsCopy()
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if _, exists := descriptions[groupName]; !exists {
			descriptions[groupName] = ""
		}
	}
	for _, pricingModel := range pricing {
		for _, groupName := range pricingModel.EnableGroup {
			if groupName == "" || groupName == "auto" || groupName == "all" {
				continue
			}
			if _, exists := descriptions[groupName]; !exists {
				descriptions[groupName] = ""
			}
		}
	}
	delete(descriptions, "auto")
	delete(descriptions, "all")
	return descriptions
}

func mergeGroupStatuses(
	pricing []model.Pricing,
	descriptions map[string]string,
	entries map[string]model_status_setting.Entry,
	includeDisabled bool,
) []GroupStatus {
	modelSets := make(map[string]map[string]struct{}, len(descriptions))
	for _, pricingModel := range pricing {
		allGroups := false
		for _, groupName := range pricingModel.EnableGroup {
			if groupName == "all" {
				allGroups = true
				break
			}
		}
		if allGroups {
			for groupName := range descriptions {
				if modelSets[groupName] == nil {
					modelSets[groupName] = make(map[string]struct{})
				}
				modelSets[groupName][pricingModel.ModelName] = struct{}{}
			}
			continue
		}
		for _, groupName := range pricingModel.EnableGroup {
			if _, visible := descriptions[groupName]; !visible {
				continue
			}
			if modelSets[groupName] == nil {
				modelSets[groupName] = make(map[string]struct{})
			}
			modelSets[groupName][pricingModel.ModelName] = struct{}{}
		}
	}

	statuses := make([]GroupStatus, 0, len(modelSets))
	for groupName, modelSet := range modelSets {
		entry, enabled := entries[groupName]
		if !enabled && !includeDisabled {
			continue
		}
		status := model_status_setting.StatusAvailable
		updatedAt := entry.UpdatedAt
		automaticStatus, observedAt, automated := automaticGroupStatus(groupName, time.Now())
		if enabled {
			status = entry.Status
			if automated && status != model_status_setting.StatusUnavailable {
				status = automaticStatus
				updatedAt = observedAt
			}
		}
		models := make([]string, 0, len(modelSet))
		for modelName := range modelSet {
			models = append(models, modelName)
		}
		sort.Slice(models, func(i, j int) bool {
			return strings.ToLower(models[i]) < strings.ToLower(models[j])
		})
		statuses = append(statuses, GroupStatus{
			GroupName:   groupName,
			Description: descriptions[groupName],
			Enabled:     enabled,
			Status:      status,
			Message:     entry.Message,
			UpdatedAt:   updatedAt,
			Models:      models,
			Automated:   automated,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return strings.ToLower(statuses[i].GroupName) < strings.ToLower(statuses[j].GroupName)
	})
	return statuses
}

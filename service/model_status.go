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
	"github.com/QuantumNous/new-api/setting/model_status_setting"
)

type ModelStatus struct {
	ModelName   string `json:"model_name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	VendorIcon  string `json:"vendor_icon,omitempty"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	UpdatedAt   int64  `json:"updated_at,omitempty"`
}

type ModelStatusUpdate struct {
	ModelName string `json:"model_name"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

var modelStatusUpdateMutex sync.Mutex

var ErrInvalidModelStatusUpdate = errors.New("invalid model status update")

func GetPublishedModelStatuses() ([]ModelStatus, error) {
	items, err := model_status_setting.ParseItems(model_status_setting.GetSetting().Items)
	if err != nil {
		return nil, err
	}
	pricing := model.GetPricing()
	return mergeModelStatuses(pricing, model.GetVendors(), items, false), nil
}

func GetManagedModelStatuses() ([]ModelStatus, error) {
	items, err := model_status_setting.ParseItems(model_status_setting.GetSetting().Items)
	if err != nil {
		return nil, err
	}
	pricing := model.GetPricing()
	return mergeModelStatuses(pricing, model.GetVendors(), items, true), nil
}

func UpdateModelStatus(update ModelStatusUpdate) error {
	update.ModelName = strings.TrimSpace(update.ModelName)
	update.Message = strings.TrimSpace(update.Message)
	if update.ModelName == "" || len(update.ModelName) > 256 {
		return fmt.Errorf("%w: invalid model name", ErrInvalidModelStatusUpdate)
	}
	if utf8.RuneCountInString(update.Message) > 500 {
		return fmt.Errorf("%w: model status message cannot exceed 500 characters", ErrInvalidModelStatusUpdate)
	}
	if update.Enabled && !model_status_setting.IsValidStatus(update.Status) {
		return fmt.Errorf("%w: invalid model status", ErrInvalidModelStatusUpdate)
	}

	modelExists := false
	for _, pricing := range model.GetPricing() {
		if pricing.ModelName == update.ModelName {
			modelExists = true
			break
		}
	}
	if !modelExists {
		return fmt.Errorf("%w: model %q is not present in the model catalog", ErrInvalidModelStatusUpdate, update.ModelName)
	}

	modelStatusUpdateMutex.Lock()
	defer modelStatusUpdateMutex.Unlock()
	items, err := model_status_setting.ParseItems(model_status_setting.GetSetting().Items)
	if err != nil {
		return err
	}
	if update.Enabled {
		items[update.ModelName] = model_status_setting.Entry{
			Status:    update.Status,
			Message:   update.Message,
			UpdatedAt: time.Now().Unix(),
		}
	} else {
		delete(items, update.ModelName)
	}
	payload, err := common.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal model status items: %w", err)
	}
	return model.UpdateOption(model_status_setting.ItemsOptionKey, string(payload))
}

func mergeModelStatuses(pricing []model.Pricing, vendors []model.PricingVendor, items map[string]model_status_setting.Entry, includeDisabled bool) []ModelStatus {
	vendorIcons := make(map[int]string, len(vendors))
	for _, vendor := range vendors {
		vendorIcons[vendor.ID] = vendor.Icon
	}
	statuses := make([]ModelStatus, 0, len(pricing))
	for _, pricingModel := range pricing {
		entry, enabled := items[pricingModel.ModelName]
		if !enabled && !includeDisabled {
			continue
		}
		status := model_status_setting.StatusAvailable
		if enabled {
			status = entry.Status
		}
		statuses = append(statuses, ModelStatus{
			ModelName:   pricingModel.ModelName,
			Description: pricingModel.Description,
			Icon:        pricingModel.Icon,
			VendorIcon:  vendorIcons[pricingModel.VendorID],
			Enabled:     enabled,
			Status:      status,
			Message:     entry.Message,
			UpdatedAt:   entry.UpdatedAt,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return strings.ToLower(statuses[i].ModelName) < strings.ToLower(statuses[j].ModelName)
	})
	return statuses
}

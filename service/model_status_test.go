package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_status_setting"
	"github.com/stretchr/testify/assert"
)

func TestMergeModelStatusesUsesCatalogAsSource(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "model-b", Description: "B", VendorID: 2},
		{ModelName: "model-a", Description: "A", VendorID: 1},
	}
	vendors := []model.PricingVendor{{ID: 1, Icon: "Claude.Color"}, {ID: 2, Icon: "OpenAI.Color"}}
	items := map[string]model_status_setting.Entry{
		"model-a":     {Status: model_status_setting.StatusUnavailable, Message: "Paused", UpdatedAt: 10},
		"stale-model": {Status: model_status_setting.StatusAvailable, UpdatedAt: 10},
	}

	public := mergeModelStatuses(pricing, vendors, items, false)
	assert.Equal(t, []ModelStatus{{
		ModelName: "model-a", Description: "A", Enabled: true,
		VendorIcon: "Claude.Color",
		Status:     model_status_setting.StatusUnavailable, Message: "Paused", UpdatedAt: 10,
	}}, public)

	managed := mergeModelStatuses(pricing, vendors, items, true)
	assert.Len(t, managed, 2)
	assert.Equal(t, "model-a", managed[0].ModelName)
	assert.Equal(t, "model-b", managed[1].ModelName)
	assert.False(t, managed[1].Enabled)
	assert.Equal(t, "OpenAI.Color", managed[1].VendorIcon)
	assert.Equal(t, model_status_setting.StatusAvailable, managed[1].Status)
}

package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_status_setting"
	"github.com/stretchr/testify/assert"
)

func TestMergeGroupStatusesUsesCatalogMembership(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "model-b", EnableGroup: []string{"compatible"}},
		{ModelName: "model-a", EnableGroup: []string{"compatible", "premium"}},
		{ModelName: "model-global", EnableGroup: []string{"all"}},
	}
	descriptions := map[string]string{
		"compatible": "Compatible pool",
		"premium":    "Premium pool",
	}
	entries := map[string]model_status_setting.Entry{
		"compatible": {Status: model_status_setting.StatusUnavailable, Message: "Cooling down", UpdatedAt: 10},
		"stale":      {Status: model_status_setting.StatusAvailable, UpdatedAt: 10},
	}

	public := mergeGroupStatuses(pricing, descriptions, entries, false)
	assert.Equal(t, []GroupStatus{{
		GroupName: "compatible", Description: "Compatible pool", Enabled: true,
		Status: model_status_setting.StatusUnavailable, Message: "Cooling down", UpdatedAt: 10,
		Models: []string{"model-a", "model-b", "model-global"},
	}}, public)

	managed := mergeGroupStatuses(pricing, descriptions, entries, true)
	assert.Len(t, managed, 2)
	assert.Equal(t, "compatible", managed[0].GroupName)
	assert.Equal(t, "premium", managed[1].GroupName)
	assert.False(t, managed[1].Enabled)
	assert.Equal(t, model_status_setting.StatusAvailable, managed[1].Status)
	assert.Equal(t, []string{"model-a", "model-global"}, managed[1].Models)
}

func TestMergeGroupStatusesDoesNotExposeGroupsOutsideViewerAccess(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "model-a", EnableGroup: []string{"compatible", "private"}},
	}
	entries := map[string]model_status_setting.Entry{
		"compatible": {Status: model_status_setting.StatusAvailable, UpdatedAt: 10},
		"private":    {Status: model_status_setting.StatusUnavailable, UpdatedAt: 10},
	}

	statuses := mergeGroupStatuses(
		pricing,
		map[string]string{"compatible": "Public pool"},
		entries,
		false,
	)

	assert.Equal(t, []GroupStatus{{
		GroupName: "compatible", Description: "Public pool", Enabled: true,
		Status: model_status_setting.StatusAvailable, UpdatedAt: 10,
		Models: []string{"model-a"},
	}}, statuses)
}

package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_status_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAutomatedGroupStatusRespectsManualUnavailableOverride(t *testing.T) {
	setting := config.GlobalConfig.Get("model_status_setting").(*model_status_setting.Setting)
	previousSources := setting.Sources
	setting.Sources = `[{"id":"source-a","name":"Relay A","url":"https://relay.example/status","enabled":true,"mappings":{"compatible":"compatible"}}]`
	modelStatusSourceRuntime.Lock()
	previousObservations := modelStatusSourceRuntime.Observations
	modelStatusSourceRuntime.Observations = map[string]modelStatusSourceObservation{
		"source-a": {Availability: map[string]bool{"compatible": true}, GeneratedAt: 20, ReceivedAt: time.Now()},
	}
	modelStatusSourceRuntime.Unlock()
	t.Cleanup(func() {
		setting.Sources = previousSources
		modelStatusSourceRuntime.Lock()
		modelStatusSourceRuntime.Observations = previousObservations
		modelStatusSourceRuntime.Unlock()
	})

	pricing := []model.Pricing{{ModelName: "model-a", EnableGroup: []string{"compatible"}}}
	descriptions := map[string]string{"compatible": "Compatible pool"}

	automatic := mergeGroupStatuses(pricing, descriptions, map[string]model_status_setting.Entry{
		"compatible": {Status: model_status_setting.StatusMaintenance, UpdatedAt: 10},
	}, false)
	require.Len(t, automatic, 1)
	assert.Equal(t, model_status_setting.StatusAvailable, automatic[0].Status)
	assert.True(t, automatic[0].Automated)
	assert.Equal(t, int64(20), automatic[0].UpdatedAt)

	manual := mergeGroupStatuses(pricing, descriptions, map[string]model_status_setting.Entry{
		"compatible": {Status: model_status_setting.StatusUnavailable, UpdatedAt: 30},
	}, false)
	require.Len(t, manual, 1)
	assert.Equal(t, model_status_setting.StatusUnavailable, manual[0].Status)
	assert.Equal(t, int64(30), manual[0].UpdatedAt)
}

func TestGroupAvailabilitySummaryContainsOnlyPublishedGroupStates(t *testing.T) {
	statuses := []GroupStatus{
		{GroupName: "cc-compatible", Status: model_status_setting.StatusAvailable, UpdatedAt: 10, Models: []string{"secret-model"}},
		{GroupName: "cc-only", Status: model_status_setting.StatusMaintenance, UpdatedAt: 20, Description: "internal"},
	}

	summary := summarizeGroupAvailability(statuses)

	assert.Equal(t, int64(20), summary.UpdatedAt)
	assert.Equal(t, map[string]string{
		"cc-compatible": model_status_setting.StatusAvailable,
		"cc-only":       model_status_setting.StatusMaintenance,
	}, summary.Groups)
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

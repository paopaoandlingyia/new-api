package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/model_status_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchModelStatusSourceUsesReadOnlyBearerContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer status-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"generated_at":20,"availability":{"compatible":true,"official":false}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	response, err := fetchModelStatusSource(context.Background(), server.Client(), model_status_setting.Source{
		ID:     "source-a",
		Name:   "Relay A",
		URL:    server.URL,
		APIKey: "status-key",
		Mappings: map[string]string{
			"cc-compatible": "compatible",
			"cc-only":       "official",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(20), response.GeneratedAt)
	assert.Equal(t, map[string]bool{"compatible": true, "official": false}, response.Availability)
}

func TestAutomaticGroupStatusUsesAnyAvailableBoundSource(t *testing.T) {
	setting := config.GlobalConfig.Get("model_status_setting").(*model_status_setting.Setting)
	previousSources := setting.Sources
	setting.Sources = `[
		{"id":"source-a","name":"Relay A","url":"https://a.example/status","enabled":true,"mappings":{"cc-only":"official"}},
		{"id":"source-b","name":"Relay B","url":"https://b.example/status","enabled":true,"mappings":{"cc-only":"primary"}}
	]`
	modelStatusSourceRuntime.Lock()
	previousObservations := modelStatusSourceRuntime.Observations
	modelStatusSourceRuntime.Observations = map[string]modelStatusSourceObservation{
		"source-a": {Availability: map[string]bool{"official": false}, GeneratedAt: 10, ReceivedAt: time.Now()},
		"source-b": {Availability: map[string]bool{"primary": true}, GeneratedAt: 20, ReceivedAt: time.Now()},
	}
	modelStatusSourceRuntime.Unlock()
	t.Cleanup(func() {
		setting.Sources = previousSources
		modelStatusSourceRuntime.Lock()
		modelStatusSourceRuntime.Observations = previousObservations
		modelStatusSourceRuntime.Unlock()
	})

	status, updatedAt, automated := automaticGroupStatus("cc-only", time.Now())

	assert.True(t, automated)
	assert.Equal(t, model_status_setting.StatusAvailable, status)
	assert.Equal(t, int64(20), updatedAt)
}

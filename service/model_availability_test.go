package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/model_availability_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectModelAvailabilityAggregatesSourcesConservatively(t *testing.T) {
	now := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer source-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"available","observed_at":` + fmt.Sprint(now.Unix()) + `,"ttl_seconds":60,"models":{"claude-opus":{"status":"unavailable"}}}`))
	}))
	t.Cleanup(server.Close)

	sources := []model_availability_setting.Source{
		{Group: "通用", URL: server.URL, Token: "source-key"},
		{Group: "专用", URL: server.URL + "/missing", Token: "wrong-key"},
	}
	result := collectModelAvailability(t.Context(), sources, server.Client(), now)
	require.Contains(t, result.Groups, "通用")
	assert.Equal(t, "available", result.Groups["通用"].Status)
	assert.Equal(t, "unavailable", result.Groups["通用"].Models["claude-opus"].Status)
	assert.NotContains(t, result.Groups, "专用")
}

func TestAggregateAvailabilityRequiresEverySourceForNegativeStatus(t *testing.T) {
	values := []upstreamAvailability{{Status: "unavailable"}}
	status, ok := aggregateAvailability(values, 2, "")
	assert.False(t, ok)
	assert.Empty(t, status)

	values = append(values, upstreamAvailability{Status: "maintenance"})
	status, ok = aggregateAvailability(values, 2, "")
	assert.True(t, ok)
	assert.Equal(t, "unavailable", status)

	values[1].Status = "available"
	status, ok = aggregateAvailability(values, 2, "")
	assert.True(t, ok)
	assert.Equal(t, "available", status)
}

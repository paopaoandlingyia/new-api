package model_status_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGroupsValidatesPublishedStatus(t *testing.T) {
	groups, err := ParseGroups(`{"compatible":{"status":"maintenance","message":"Scheduled work","updated_at":1}}`)
	require.NoError(t, err)
	assert.Equal(t, StatusMaintenance, groups["compatible"].Status)
	assert.Equal(t, "Scheduled work", groups["compatible"].Message)
}

func TestParseGroupsRejectsInvalidPublishedStatus(t *testing.T) {
	tests := []string{
		`{"compatible":{"status":"unknown","updated_at":1}}`,
		`{"":{"status":"available","updated_at":1}}`,
		`{"compatible":{"status":"available","updated_at":0}}`,
	}
	for _, input := range tests {
		_, err := ParseGroups(input)
		assert.Error(t, err)
	}
}

func TestParseSourcesValidatesGenericAvailabilityBindings(t *testing.T) {
	sources, err := ParseSources(`[{"id":"source-a","name":"Relay A","url":"https://relay.example/ops/v1/availability","api_key":"secret","enabled":true,"mappings":{"cc-compatible":"compatible","cc-only":"official"}}]`)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "official", sources[0].Mappings["cc-only"])
}

func TestParseSourcesRejectsInvalidConfiguration(t *testing.T) {
	tests := []string{
		`[{"id":"source-a","name":"Relay A","url":"not-a-url","enabled":true,"mappings":{"cc-only":"official"}}]`,
		`[{"id":"source-a","name":"Relay A","url":"https://a.example/status","enabled":true,"mappings":{}}]`,
		`[{"id":"source-a","name":"Relay A","url":"https://a.example/status","enabled":true,"mappings":{"cc-only":"official"}},{"id":"source-a","name":"Relay B","url":"https://b.example/status","enabled":true,"mappings":{"cc-only":"official"}}]`,
	}
	for _, input := range tests {
		_, err := ParseSources(input)
		assert.Error(t, err)
	}
}

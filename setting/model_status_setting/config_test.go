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

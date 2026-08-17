package model_status_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseItemsValidatesPublishedStatus(t *testing.T) {
	items, err := ParseItems(`{"claude-sonnet-5":{"status":"maintenance","message":"Scheduled work","updated_at":1}}`)
	require.NoError(t, err)
	assert.Equal(t, StatusMaintenance, items["claude-sonnet-5"].Status)
	assert.Equal(t, "Scheduled work", items["claude-sonnet-5"].Message)
}

func TestParseItemsRejectsInvalidPublishedStatus(t *testing.T) {
	tests := []string{
		`{"claude-sonnet-5":{"status":"unknown","updated_at":1}}`,
		`{"":{"status":"available","updated_at":1}}`,
		`{"claude-sonnet-5":{"status":"available","updated_at":0}}`,
	}
	for _, input := range tests {
		_, err := ParseItems(input)
		assert.Error(t, err)
	}
}

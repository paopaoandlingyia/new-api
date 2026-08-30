package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeCountTokensRequestBodyPreservesInputFields(t *testing.T) {
	rawBody := []byte(`{
		"model":"claude-client-model",
		"messages":[{"role":"user","content":"hello"}],
		"system":[{"type":"text","text":"system"}],
		"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
		"context_management":{"edits":[]},
		"future_field":{"enabled":true}
	}`)

	jsonData, err := buildClaudeCountTokensRequestBody(rawBody, "claude-upstream-model")
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, common.Unmarshal(jsonData, &body))
	assert.Equal(t, "claude-upstream-model", body["model"])
	assert.Contains(t, body, "messages")
	assert.Contains(t, body, "system")
	assert.Contains(t, body, "tools")
	assert.Contains(t, body, "context_management")
	assert.Contains(t, body, "future_field")
}

func TestSanitizeClaudeCountTokensRequestBodyRunsAfterParamOverride(t *testing.T) {
	jsonData := []byte(`{
		"model":"claude-test",
		"messages":[{"role":"user","content":"hello"}],
		"future_field":true
	}`)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ParamOverride: map[string]any{
			"temperature":    0.7,
			"top_p":          0.9,
			"top_k":          10,
			"stream":         true,
			"stop_sequences": []string{"stop"},
			"stop":           "stop",
			"max_tokens":     100,
			"added_field":    "kept",
		},
	}}

	overridden, err := relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
	require.NoError(t, err)
	sanitized, err := sanitizeClaudeCountTokensRequestBody(overridden)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, common.Unmarshal(sanitized, &body))
	for _, field := range claudeCountTokensGenerationFields {
		assert.NotContains(t, body, field)
	}
	assert.Equal(t, "kept", body["added_field"])
	assert.Equal(t, true, body["future_field"])
}

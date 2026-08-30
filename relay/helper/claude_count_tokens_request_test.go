package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidateClaudeCountTokensRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{
		"model":"claude-sonnet-4-5",
		"messages":[{"role":"user","content":"hello"}],
		"future_field":{"enabled":true}
	}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	request, err := GetAndValidateClaudeCountTokensRequest(c)
	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-5", request.Model)
	require.Len(t, request.Messages, 1)

	var rawBody map[string]any
	require.NoError(t, common.Unmarshal(request.RawBody, &rawBody))
	assert.Contains(t, rawBody, "future_field")
}

func TestGetAndValidateClaudeCountTokensRequestRequiresModelAndMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{name: "missing model", body: `{"messages":[{"role":"user","content":"hello"}]}`, wantError: "field model is required"},
		{name: "missing messages", body: `{"model":"claude-sonnet-4-5"}`, wantError: "field messages is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			_, err := GetAndValidateClaudeCountTokensRequest(c)
			require.EqualError(t, err, tt.wantError)
		})
	}
}

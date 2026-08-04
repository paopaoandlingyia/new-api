package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildModelProbeRequestUsesShortPromptWithoutTokenCap(t *testing.T) {
	for _, endpointType := range []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeAnthropic,
		constant.EndpointTypeGemini,
	} {
		request, ok := buildTestRequest("test-model", string(endpointType), nil, false, true).(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		require.Len(t, request.Messages, 1)
		assert.Equal(t, modelProbePrompt, request.Messages[0].Content)
		assert.Nil(t, request.MaxTokens)
		assert.Nil(t, request.MaxCompletionTokens)
	}

	responsesRequest, ok := buildTestRequest(
		"test-model",
		string(constant.EndpointTypeOpenAIResponse),
		nil,
		false,
		true,
	).(*dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.JSONEq(t, `[{"role":"user","content":"just say hi"}]`, string(responsesRequest.Input))
	assert.Nil(t, responsesRequest.MaxOutputTokens)
}

func TestBuildRegularChannelTestRequestKeepsExistingTokenCap(t *testing.T) {
	request, ok := buildTestRequest(
		"test-model",
		string(constant.EndpointTypeOpenAI),
		nil,
		false,
		false,
	).(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.NotNil(t, request.MaxTokens)
	assert.EqualValues(t, 16, *request.MaxTokens)
	assert.Equal(t, "hi", request.Messages[0].Content)
}

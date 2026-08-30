package claude

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLClaudeCountTokens(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeClaudeCountTokens,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.anthropic.com",
		},
	}

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://api.anthropic.com/v1/messages/count_tokens", requestURL)
}

func TestGetRequestURLClaudeCountTokensWithBetaQuery(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:         relayconstant.RelayModeClaudeCountTokens,
		IsClaudeBetaQuery: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.anthropic.com",
		},
	}

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://api.anthropic.com/v1/messages/count_tokens?beta=true", requestURL)
}

package dto

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// ClaudeCountTokensRequest contains the fields needed to validate and route an
// Anthropic count_tokens request. RawBody preserves input fields that newer
// Anthropic API versions may add before this gateway knows about them.
type ClaudeCountTokensRequest struct {
	Model    string          `json:"model"`
	Messages []ClaudeMessage `json:"messages"`
	RawBody  json.RawMessage `json:"-"`
}

func (r *ClaudeCountTokensRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer}
}

func (r *ClaudeCountTokensRequest) IsStream(_ *http.Request) bool {
	return false
}

func (r *ClaudeCountTokensRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

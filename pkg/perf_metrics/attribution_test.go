package perfmetrics

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/stretchr/testify/assert"
)

// 模型可用性统计只接受平台/上游责任的失败。这里锁定的是归因契约本身：
// 调用方责任的失败必须被剔除，否则用户的错误请求会拉低模型可用率。
func TestAttributeRelayFailure(t *testing.T) {
	cases := []struct {
		name string
		err  *types.NewAPIError
		want FailureAttribution
	}{
		{
			name: "nil error is not sampled",
			err:  nil,
			want: AttributionClient,
		},
		{
			name: "upstream content policy refusal",
			err:  types.NewErrorWithStatusCode(errors.New("content blocked"), "content_policy_violation", http.StatusBadRequest),
			want: AttributionClient,
		},
		{
			name: "content filter reported with a misleading 5xx status",
			err:  types.NewErrorWithStatusCode(errors.New("filtered"), "content_filter", http.StatusInternalServerError),
			want: AttributionClient,
		},
		{
			name: "context length exceeded",
			err:  types.NewErrorWithStatusCode(errors.New("too long"), "context_length_exceeded", http.StatusBadRequest),
			want: AttributionClient,
		},
		{
			name: "unknown upstream code with 400 status",
			err:  types.NewErrorWithStatusCode(errors.New("bad param"), "unknown_error", http.StatusBadRequest),
			want: AttributionClient,
		},
		{
			name: "request body too large",
			err:  types.NewErrorWithStatusCode(errors.New("too large"), types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge),
			want: AttributionClient,
		},
		{
			name: "insufficient user quota",
			err:  types.NewError(errors.New("no quota"), types.ErrorCodeInsufficientUserQuota),
			want: AttributionClient,
		},
		{
			name: "local sensitive words policy",
			err:  types.NewError(errors.New("blocked"), types.ErrorCodeSensitiveWordsDetected),
			want: AttributionClient,
		},
		{
			name: "access denied by local authorization",
			err:  types.NewErrorWithStatusCode(errors.New("denied"), types.ErrorCodeAccessDenied, http.StatusForbidden),
			want: AttributionClient,
		},
		{
			name: "no available channel",
			err:  types.NewError(errors.New("no channel"), types.ErrorCodeGetChannelFailed),
			want: AttributionPlatform,
		},
		{
			name: "channel key invalid",
			err:  types.NewErrorWithStatusCode(errors.New("bad key"), types.ErrorCodeChannelInvalidKey, http.StatusUnauthorized),
			want: AttributionPlatform,
		},
		{
			name: "upstream server error",
			err:  types.NewErrorWithStatusCode(errors.New("boom"), "unknown_error", http.StatusInternalServerError),
			want: AttributionPlatform,
		},
		{
			name: "upstream rate limit",
			err:  types.NewErrorWithStatusCode(errors.New("slow down"), "rate_limit_exceeded", http.StatusTooManyRequests),
			want: AttributionPlatform,
		},
		{
			name: "upstream rejects our credential",
			err:  types.NewErrorWithStatusCode(errors.New("unauthorized"), "invalid_api_key", http.StatusUnauthorized),
			want: AttributionPlatform,
		},
		{
			name: "model missing upstream",
			err:  types.NewErrorWithStatusCode(errors.New("no model"), types.ErrorCodeModelNotFound, http.StatusNotFound),
			want: AttributionPlatform,
		},
		{
			name: "upstream connection failure",
			err:  types.NewError(errors.New("dial tcp: timeout"), types.ErrorCodeDoRequestFailed),
			want: AttributionPlatform,
		},
		{
			name: "upstream returned an empty response",
			err:  types.NewError(errors.New("empty"), types.ErrorCodeEmptyResponse),
			want: AttributionPlatform,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AttributeRelayFailure(tc.err))
		})
	}
}

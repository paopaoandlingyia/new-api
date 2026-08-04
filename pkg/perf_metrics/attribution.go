package perfmetrics

import (
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// FailureAttribution 描述一次失败的中继请求该归谁的责任，决定它是否进入模型可用性统计。
type FailureAttribution int

const (
	// AttributionPlatform 平台或上游的责任：无可用渠道、密钥失效、连接失败、
	// 上游 5xx、空响应、上游限流。记为一次失败样本。
	AttributionPlatform FailureAttribution = iota
	// AttributionClient 调用方或本站策略的责任：请求参数非法、超长上下文、
	// 内容被拒、额度不足、访问被拒。这类请求完全不进入样本 —— 既不算成功也不
	// 算失败。把它们计为失败会让用户自己的错误拉低模型可用率，而计为成功又会
	// 掩盖真实故障，唯一正确的处理是不采样。
	AttributionClient
)

// 上游按 OpenAI 协议回传的错误会经 types.WithOpenAIError 把上游自己的 code
// 字符串填进 errorCode，所以绝大多数真实上游错误不会命中下面的内部错误码分支，
// 而是落到状态码兜底。这几个 code 单独固定归因，因为它们语义明确且各家上游
// 附带的状态码并不一致。
const (
	upstreamCodeContentFilter         types.ErrorCode = "content_filter"
	upstreamCodeContentPolicy         types.ErrorCode = "content_policy_violation"
	upstreamCodeContextLengthExceeded types.ErrorCode = "context_length_exceeded"
)

// AttributeRelayFailure 判断一次中继失败是否应计入模型可用性统计。
//
// 只有平台/上游责任的失败才代表"这个模型现在不好用"。调用方责任的失败与模型
// 健康无关，必须从样本里剔除，否则可用率会被用户的错误请求污染。
func AttributeRelayFailure(err *types.NewAPIError) FailureAttribution {
	if err == nil {
		// 无错误时不应记录失败样本，按不采样处理。
		return AttributionClient
	}

	// channel: 前缀的错误码统一表示渠道或密钥层面的故障。
	if types.IsChannelError(err) {
		return AttributionPlatform
	}

	switch err.GetErrorCode() {
	case types.ErrorCodeInvalidRequest,
		types.ErrorCodeBadRequestBody,
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeConvertRequestFailed,
		types.ErrorCodeAccessDenied,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodePromptBlocked,
		types.ErrorCodeViolationFeeGrokCSAM,
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed,
		types.ErrorCodeCountTokenFailed,
		upstreamCodeContentFilter,
		upstreamCodeContentPolicy,
		upstreamCodeContextLengthExceeded:
		return AttributionClient
	case types.ErrorCodeGetChannelFailed,
		types.ErrorCodeDoRequestFailed,
		types.ErrorCodeBadResponseStatusCode,
		types.ErrorCodeBadResponse,
		types.ErrorCodeBadResponseBody,
		types.ErrorCodeReadResponseBodyFailed,
		types.ErrorCodeEmptyResponse,
		types.ErrorCodeAwsInvokeError,
		types.ErrorCodeModelNotFound,
		types.ErrorCodeInvalidApiType,
		types.ErrorCodeJsonMarshalFailed,
		types.ErrorCodeQueryDataError,
		types.ErrorCodeUpdateDataError:
		return AttributionPlatform
	}

	// 未知错误码退回状态码判断。只有明确表示"请求本身有问题"的状态码归给调用方；
	// 401/403/408/429 以及 5xx 都说明请求没毛病而平台没能服务它，算平台故障。
	// 本站自己的限流在中间件阶段就返回了，走不到这里，所以 429 只可能来自上游。
	switch err.StatusCode {
	case http.StatusBadRequest,
		http.StatusRequestEntityTooLarge,
		http.StatusUnprocessableEntity:
		return AttributionClient
	}
	return AttributionPlatform
}

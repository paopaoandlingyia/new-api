// Package modelprobe 保存模型可用性主动探测的结果并派生对外状态。
//
// 与 pkg/perf_metrics 的区别：perf_metrics 统计真实用户流量，是管理员运营数据；
// modelprobe 的样本全部由本站自己发出，因此不含任何用户使用信息，可以安全地
// 公开展示。探测的执行在 controller/model_probe.go，本包只负责状态与存储。
package modelprobe

// Status 是对外展示的离散状态。刻意不暴露成功率百分比或样本数量：状态灯只需要
// 回答"能不能用"，给出可用率反而会引出"分母是多少"这类无意义的追问。
type Status string

const (
	// StatusOperational 最近一次探测通过，且观察窗口内没有失败。
	StatusOperational Status = "operational"
	// StatusDegraded 最近一次探测通过但窗口内有过失败，或刚失败但未达到故障阈值。
	StatusDegraded Status = "degraded"
	// StatusOutage 连续失败达到阈值。
	StatusOutage Status = "outage"
	// StatusUnmonitored 未纳入监测：未被管理员选中、非文本模型，或在探测分组下无可用渠道。
	StatusUnmonitored Status = "unmonitored"
	// StatusUnknown 纳入了监测但还没有探测结果（例如刚启动）。
	StatusUnknown Status = "unknown"
)

// Record 是单个模型的探测状态，存储在 Redis 或进程内存里。
// 它含有渠道 ID 和上游错误信息，只能给管理员看，对外要经 Public 投影。
type Record struct {
	ModelName           string `json:"model_name"`
	Monitored           bool   `json:"monitored"`
	LastProbeAt         int64  `json:"last_probe_at,omitempty"`
	LastSuccessAt       int64  `json:"last_success_at,omitempty"`
	LatencyMs           int64  `json:"latency_ms,omitempty"`
	ConsecutiveFailures int    `json:"consecutive_failures,omitempty"`
	LastError           string `json:"last_error,omitempty"`
	ChannelID           int    `json:"channel_id,omitempty"`
	// Recent 是最近若干次探测结果的环形记录，最新的在末尾。用于区分"一直好"
	// 和"刚恢复"，后者应显示为波动而不是正常。
	Recent []bool `json:"recent,omitempty"`
}

// PublicStatus 是模型广场使用的投影，只保留展示状态灯所需的字段。
type PublicStatus struct {
	ModelName   string `json:"model_name"`
	Status      Status `json:"status"`
	LastProbeAt int64  `json:"last_probe_at,omitempty"`
	LatencyMs   int64  `json:"latency_ms,omitempty"`
	Recent      []bool `json:"recent,omitempty"`
}

// degradedFailureCount 是"已恢复但仍算波动"所需的窗口内失败次数。
//
// 取 2 而不是 1 是关键：观察窗口是 Recent 的长度乘以探测间隔（默认 6 × 10 分钟
// ＝ 一小时），如果一次孤立失败就判波动，模型在恢复后仍会黄整整一小时。那正是
// 这套改造要消灭的"可用率九成还亮红灯"。波动应当表示"真的在抖"，即窗口内反复
// 失败；单次抖动恢复后就该回到可用。
const degradedFailureCount = 2

// Status 派生对外状态。阈值在读取时计算而不是写入时固化，这样管理员调整阈值
// 后不需要等下一轮探测才生效。
func (r Record) Status(outageThreshold int) Status {
	if !r.Monitored {
		return StatusUnmonitored
	}
	if r.LastProbeAt == 0 {
		return StatusUnknown
	}
	if outageThreshold < 1 {
		outageThreshold = 1
	}
	if r.ConsecutiveFailures >= outageThreshold {
		return StatusOutage
	}
	// 当前正处于失败状态但还没到故障阈值，直接算波动：此刻确实调不通。
	if r.ConsecutiveFailures > 0 {
		return StatusDegraded
	}
	if r.recentFailures() >= degradedFailureCount {
		return StatusDegraded
	}
	return StatusOperational
}

func (r Record) recentFailures() int {
	failures := 0
	for _, ok := range r.Recent {
		if !ok {
			failures++
		}
	}
	return failures
}

func (r Record) Public(outageThreshold int) PublicStatus {
	return PublicStatus{
		ModelName:   r.ModelName,
		Status:      r.Status(outageThreshold),
		LastProbeAt: r.LastProbeAt,
		LatencyMs:   r.LatencyMs,
		// Recent 可以公开：探测按固定节奏发出，与用户是否使用该模型无关，
		// 所以这串结果不透露任何使用情况，只是让状态灯能显示最近的走势。
		Recent: r.Recent,
	}
}

// ApplyResult 把一次探测结果并入记录，返回更新后的记录。
// errMessage 为空表示探测成功。
func ApplyResult(previous Record, modelName string, now int64, latencyMs int64, errMessage string, channelID int, ringSize int) Record {
	updated := previous
	updated.ModelName = modelName
	updated.Monitored = true
	updated.LastProbeAt = now
	updated.ChannelID = channelID

	success := errMessage == ""
	if success {
		updated.LastSuccessAt = now
		updated.LatencyMs = latencyMs
		updated.ConsecutiveFailures = 0
		updated.LastError = ""
	} else {
		updated.ConsecutiveFailures = previous.ConsecutiveFailures + 1
		updated.LastError = errMessage
		// 失败时不覆盖延迟，保留最后一次成功的数值，否则超时会把延迟污染成超时上限。
	}

	if ringSize < 1 {
		ringSize = 1
	}
	updated.Recent = append(append([]bool{}, previous.Recent...), success)
	if len(updated.Recent) > ringSize {
		updated.Recent = updated.Recent[len(updated.Recent)-ringSize:]
	}
	return updated
}

// Unmonitored 生成一条"不监测"记录，用于未纳入监测或探测分组下不可用的模型。
// 这类模型必须与"故障"区分开：它们不是坏了，而是压根没探。
func Unmonitored(modelName string) Record {
	return Record{ModelName: modelName, Monitored: false}
}

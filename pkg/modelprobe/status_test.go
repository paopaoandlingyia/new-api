package modelprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 状态派生是状态灯的对外契约：未监测、无数据、正常、波动、故障必须严格区分。
// 尤其"未监测"不能显示成"故障"——被排除的模型不是坏了，而是压根没探。
func TestRecordStatus(t *testing.T) {
	cases := []struct {
		name      string
		record    Record
		threshold int
		want      Status
	}{
		{
			name:      "excluded model is unmonitored, not an outage",
			record:    Unmonitored("claude-opus-5"),
			threshold: 2,
			want:      StatusUnmonitored,
		},
		{
			name:      "monitored but never probed yet",
			record:    Record{ModelName: "m", Monitored: true},
			threshold: 2,
			want:      StatusUnknown,
		},
		{
			name:      "all recent probes passed",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, Recent: []bool{true, true, true}},
			threshold: 2,
			want:      StatusOperational,
		},
		{
			// 一次孤立失败恢复后必须回到可用。观察窗口有一小时，若单次抖动就判
			// 波动，模型恢复后仍会黄整整一小时，等于重演"可用率九成还亮红灯"。
			name:      "single isolated failure recovers to available",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, Recent: []bool{false, true, true, true, true}},
			threshold: 2,
			want:      StatusOperational,
		},
		{
			name:      "repeated failures in the window still read as degraded",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, Recent: []bool{false, true, false, true, true}},
			threshold: 2,
			want:      StatusDegraded,
		},
		{
			name:      "single failure below the outage threshold",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, ConsecutiveFailures: 1, Recent: []bool{true, false}},
			threshold: 2,
			want:      StatusDegraded,
		},
		{
			name:      "consecutive failures reach the threshold",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, ConsecutiveFailures: 2, Recent: []bool{false, false}},
			threshold: 2,
			want:      StatusOutage,
		},
		{
			name:      "threshold of one turns any failure into an outage",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, ConsecutiveFailures: 1, Recent: []bool{true, false}},
			threshold: 1,
			want:      StatusOutage,
		},
		{
			name:      "invalid threshold falls back to one",
			record:    Record{ModelName: "m", Monitored: true, LastProbeAt: 100, ConsecutiveFailures: 1, Recent: []bool{false}},
			threshold: 0,
			want:      StatusOutage,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.record.Status(tc.threshold))
		})
	}
}

// 失败不能覆盖延迟：否则一次超时会把展示的延迟污染成超时上限，而那不是这个模型
// 正常工作时的延迟。
func TestApplyResultKeepsLastSuccessfulLatency(t *testing.T) {
	first := ApplyResult(Record{}, "m", 100, 800, "", 7, 6)
	require.Equal(t, int64(800), first.LatencyMs)
	require.Equal(t, int64(100), first.LastSuccessAt)
	require.Equal(t, 0, first.ConsecutiveFailures)

	failed := ApplyResult(first, "m", 200, 30000, "status_code=503, upstream down", 7, 6)
	assert.Equal(t, int64(800), failed.LatencyMs, "失败不应覆盖延迟")
	assert.Equal(t, int64(100), failed.LastSuccessAt, "失败不应推进最后成功时间")
	assert.Equal(t, 1, failed.ConsecutiveFailures)
	assert.Equal(t, "status_code=503, upstream down", failed.LastError)

	recovered := ApplyResult(failed, "m", 300, 900, "", 7, 6)
	assert.Equal(t, int64(900), recovered.LatencyMs)
	assert.Equal(t, 0, recovered.ConsecutiveFailures, "成功后必须清零连续失败计数")
	assert.Empty(t, recovered.LastError, "成功后必须清掉上一次的错误")
	assert.Equal(
		t,
		StatusOperational,
		recovered.Status(2),
		"单次失败恢复后应回到可用，不能因为窗口里还留着那次失败继续显示波动",
	)
}

// 环形记录必须有界，否则长期运行会让单条记录无限增长。
func TestApplyResultRingIsBounded(t *testing.T) {
	record := Record{}
	for i := 0; i < 20; i++ {
		record = ApplyResult(record, "m", int64(i), 100, "", 1, 6)
	}
	assert.Len(t, record.Recent, 6)

	// 环形记录不能与前一条共享底层数组，否则历史记录会被后续探测就地改写。
	base := ApplyResult(Record{}, "m", 1, 100, "", 1, 6)
	branchA := ApplyResult(base, "m", 2, 100, "boom", 1, 6)
	branchB := ApplyResult(base, "m", 2, 100, "", 1, 6)
	assert.Equal(t, []bool{true, false}, branchA.Recent)
	assert.Equal(t, []bool{true, true}, branchB.Recent)
}

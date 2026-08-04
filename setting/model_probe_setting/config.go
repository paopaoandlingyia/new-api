package model_probe_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// ModelProbeSetting 控制模型可用性主动探测。探测按模型（而非按渠道）进行，
// 走真实的分发选路，所以它回答的是"用户现在调这个模型能不能通"。
type ModelProbeSetting struct {
	Enabled          bool     `json:"enabled"`
	IntervalMinutes  float64  `json:"interval_minutes"`
	Group            string   `json:"group"`
	ExcludedModels   []string `json:"excluded_models"`
	OutageThreshold  int      `json:"outage_threshold"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
	DegradedRingSize int      `json:"degraded_ring_size"`
}

// 默认关闭：探测会向上游发真实请求并产生真实费用，这种行为不应由配置默认值
// 自行开启，必须由管理员显式打开。
var modelProbeSetting = ModelProbeSetting{
	Enabled:          false,
	IntervalMinutes:  10,
	Group:            "default",
	ExcludedModels:   []string{},
	OutageThreshold:  2,
	TimeoutSeconds:   30,
	DegradedRingSize: 6,
}

func init() {
	config.GlobalConfig.Register("model_probe_setting", &modelProbeSetting)
}

func GetSetting() ModelProbeSetting {
	return modelProbeSetting
}

// GetIntervalMinutes 返回探测间隔，下限 1 分钟，避免误配成 0 导致空转。
func GetIntervalMinutes() float64 {
	if modelProbeSetting.IntervalMinutes < 1 {
		return 1
	}
	return modelProbeSetting.IntervalMinutes
}

// GetProbeGroup 返回探测使用的用户分组。状态灯只代表这一个分组下的可用性。
func GetProbeGroup() string {
	group := strings.TrimSpace(modelProbeSetting.Group)
	if group == "" {
		return "default"
	}
	return group
}

func GetOutageThreshold() int {
	if modelProbeSetting.OutageThreshold < 1 {
		return 1
	}
	return modelProbeSetting.OutageThreshold
}

func GetTimeoutSeconds() int {
	if modelProbeSetting.TimeoutSeconds < 1 {
		return 30
	}
	return modelProbeSetting.TimeoutSeconds
}

func GetDegradedRingSize() int {
	if modelProbeSetting.DegradedRingSize < 1 {
		return 1
	}
	return modelProbeSetting.DegradedRingSize
}

// IsModelExcluded 判断模型是否被管理员从监测中排除。名单是排除制而非纳入制：
// 漏配一个新模型只会多探一次，而漏勾会让状态灯永远不亮，属于静默失败。
func IsModelExcluded(modelName string) bool {
	for _, excluded := range modelProbeSetting.ExcludedModels {
		if strings.EqualFold(strings.TrimSpace(excluded), modelName) {
			return true
		}
	}
	return false
}

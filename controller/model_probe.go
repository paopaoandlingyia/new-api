package controller

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/modelprobe"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_probe_setting"

	"github.com/gin-gonic/gin"
)

// 模型可用性探测。与 channel_test 的分工：channel_test 按渠道探、负责自动禁用与
// 恢复，是管理员运维视角；model_probe 按模型探、走真实分发选路，回答"用户现在
// 调这个模型能不能通"，只写状态不动渠道。
//
// 只探文本模型：非对话类端点的请求体结构完全不同，需要各自的探测器。
var probeEndpointPriority = []constant.EndpointType{
	constant.EndpointTypeOpenAI,
	constant.EndpointTypeAnthropic,
	constant.EndpointTypeGemini,
	constant.EndpointTypeOpenAIResponse,
	constant.EndpointTypeOpenAIResponseCompact,
}

// probeErrorMessageLimit 限制存储的错误摘要长度，避免上游返回的长报文撑大 Redis 记录。
const probeErrorMessageLimit = 300

// degradedIntervalMinutes 是存在失败模型时的加速探测间隔。故障期间才需要高时间
// 分辨率，平时 10 分钟足够。
const degradedIntervalMinutes = 2

// hasFailingModel 由每轮探测结束时写入，供 Interval() 判断是否进入加速模式。
// 用原子标记而不是每次调度都读一遍存储，避免调度器频繁访问 Redis。
var hasFailingModel atomic.Bool

type modelProbeSummary struct {
	Probed      int `json:"probed"`
	Operational int `json:"operational"`
	Failed      int `json:"failed"`
	Unmonitored int `json:"unmonitored"`
	TotalModels int `json:"total_models"`
}

type modelProbeHandler struct{}

func (modelProbeHandler) Type() string { return model.SystemTaskTypeModelProbe }

func (modelProbeHandler) Enabled() bool { return model_probe_setting.GetSetting().Enabled }

func (modelProbeHandler) Interval() time.Duration {
	minutes := model_probe_setting.GetIntervalMinutes()
	if hasFailingModel.Load() && minutes > degradedIntervalMinutes {
		minutes = degradedIntervalMinutes
	}
	// ±15% 抖动，避免每轮在同一秒集中打同一家上游，也避免探针流量呈现完全固定的周期。
	jitter := 1 + (rand.Float64()*0.3 - 0.15)
	return time.Duration(minutes * jitter * float64(time.Minute))
}

func (modelProbeHandler) NewPayload() any { return nil }

func (modelProbeHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	summary, err := runModelProbeTask(ctx, service.NewSystemTaskProgressReporter(task, runnerID))
	if err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, summary, nil)
}

// EnqueueModelProbeOnStartup 在启动时补一轮探测。
//
// 调度器只在"距上次任务超过一个间隔"时才建新任务，而没有 Redis 时探测状态存在
// 进程内存里、重启即清空。两者叠加会让重启后的状态页空白最长一个探测间隔。这里
// 在存储为空时主动入队一次，把空窗压到一轮探测的耗时。
func EnqueueModelProbeOnStartup() {
	if !model_probe_setting.GetSetting().Enabled {
		return
	}
	if len(modelprobe.LoadAll()) > 0 {
		return
	}
	if _, _, err := service.EnqueueSystemTask(model.SystemTaskTypeModelProbe, nil); err != nil {
		common.SysError("model probe: failed to enqueue startup probe: " + err.Error())
	}
}

// runModelProbeTask 执行一轮全量探测：对探测分组下的每个文本模型发一次真实请求，
// 把结果写入 modelprobe 存储，并清理已经下线的模型。
func runModelProbeTask(ctx context.Context, report func(processed, total int)) (modelProbeSummary, error) {
	summary := modelProbeSummary{}
	if ctx == nil {
		ctx = context.Background()
	}
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return summary, err
	}

	// 预热定价缓存，GetModelSupportEndpointTypes 依赖它判断模型的端点类型。
	_ = model.GetPricing()

	group := model_probe_setting.GetProbeGroup()
	models := model.GetGroupEnabledModels(group)
	sort.Strings(models)
	summary.TotalModels = len(models)

	// 只保留本轮仍在分组内的模型。模型的全部渠道被禁用后会从 abilities 中消失，
	// 模型广场也不再展示它，因此清理掉而不是留一盏红灯。
	keep := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		keep[modelName] = struct{}{}
	}

	ringSize := model_probe_setting.GetDegradedRingSize()
	timeout := time.Duration(model_probe_setting.GetTimeoutSeconds()) * time.Second
	existing := modelprobe.LoadAll()
	failing := false

	for index, modelName := range models {
		if ctx.Err() != nil {
			break
		}
		if report != nil {
			report(index, len(models))
		}

		endpointType, requestPath, probeable := resolveProbeEndpoint(modelName)
		if !probeable || model_probe_setting.IsModelExcluded(modelName) {
			modelprobe.Save(modelprobe.Unmonitored(modelName))
			summary.Unmonitored++
			continue
		}

		latencyMs, channelID, errMessage := probeModel(ctx, timeout, testUserID, group, modelName, endpointType, requestPath)
		record := modelprobe.ApplyResult(existing[modelName], modelName, common.GetTimestamp(), latencyMs, errMessage, channelID, ringSize)
		modelprobe.Save(record)

		summary.Probed++
		if errMessage == "" {
			summary.Operational++
		} else {
			summary.Failed++
			failing = true
		}

		if common.RequestInterval > 0 {
			select {
			case <-ctx.Done():
				return summary, nil
			case <-time.After(common.RequestInterval):
			}
		}
	}

	if ctx.Err() == nil {
		modelprobe.Prune(keep)
		hasFailingModel.Store(failing)
		if report != nil {
			report(len(models), len(models))
		}
	}
	return summary, nil
}

// probeModel 对单个模型发一次探测请求。渠道由真实的分发选路挑选，执行则复用
// channel_test 的请求构造，因此探测请求与用户请求走同一套适配器逻辑。
func probeModel(ctx context.Context, timeout time.Duration, testUserID int, group string, modelName string, endpointType string, requestPath string) (int64, int, string) {
	channel, err := model.GetRandomSatisfiedChannel(group, modelName, 0, requestPath)
	if err != nil {
		return 0, 0, truncateProbeError(fmt.Sprintf("选取渠道失败: %s", err.Error()))
	}
	if channel == nil {
		return 0, 0, fmt.Sprintf("分组 %s 下没有可用渠道", group)
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result := testChannel(probeCtx, channel, testUserID, modelName, endpointType, false)
	latencyMs := time.Since(start).Milliseconds()

	if result.newAPIError != nil {
		return latencyMs, channel.Id, truncateProbeError(result.newAPIError.MaskSensitiveErrorWithStatusCode())
	}
	if result.localErr != nil {
		return latencyMs, channel.Id, truncateProbeError(result.localErr.Error())
	}
	return latencyMs, channel.Id, ""
}

// resolveProbeEndpoint 选出探测该模型使用的端点。返回 false 表示这不是文本模型，
// 不纳入监测。
func resolveProbeEndpoint(modelName string) (string, string, bool) {
	supported := model.GetModelSupportEndpointTypes(modelName)
	for _, candidate := range probeEndpointPriority {
		for _, supportedType := range supported {
			if supportedType != candidate {
				continue
			}
			endpointInfo, ok := common.GetDefaultEndpointInfo(candidate)
			if !ok {
				continue
			}
			return string(candidate), endpointInfo.Path, true
		}
	}
	return "", "", false
}

func truncateProbeError(message string) string {
	if len(message) <= probeErrorMessageLimit {
		return message
	}
	return message[:probeErrorMessageLimit]
}

// GetModelProbeStatus 返回模型广场使用的公开状态。探测样本全部由本站自己发出，
// 不含任何用户使用信息，因此可以公开；但渠道 ID 与上游错误详情只给管理员。
func GetModelProbeStatus(c *gin.Context) {
	threshold := model_probe_setting.GetOutageThreshold()
	records := modelprobe.LoadAll()
	statuses := make([]modelprobe.PublicStatus, 0, len(records))
	for _, record := range records {
		statuses = append(statuses, record.Public(threshold))
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ModelName < statuses[j].ModelName
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":  model_probe_setting.GetSetting().Enabled,
			"statuses": statuses,
		},
	})
}

// GetModelProbeAdminStatus 返回带渠道与错误详情的完整状态，供管理页的运维表格使用。
func GetModelProbeAdminStatus(c *gin.Context) {
	threshold := model_probe_setting.GetOutageThreshold()
	records := modelprobe.LoadAll()
	type adminStatus struct {
		modelprobe.Record
		Status modelprobe.Status `json:"status"`
	}
	statuses := make([]adminStatus, 0, len(records))
	for _, record := range records {
		statuses = append(statuses, adminStatus{Record: record, Status: record.Status(threshold)})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ModelName < statuses[j].ModelName
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"setting":  model_probe_setting.GetSetting(),
			"statuses": statuses,
		},
	})
}

// TriggerModelProbe 手动触发一轮探测。与定时运行共用同一个系统任务类型，因此
// 已有任务在跑时不会重复执行。
func TriggerModelProbe(c *gin.Context) {
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeModelProbe, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !created {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "模型探测任务已在运行中",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"task_id": task.TaskID},
	})
}

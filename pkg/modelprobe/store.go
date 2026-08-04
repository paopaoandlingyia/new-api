package modelprobe

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// 探测结果不落库。Redis 可用时以 Redis 为准，这样多实例共享同一份视图；没有
// Redis 时退回进程内存，单实例部署完全够用。这与 pkg/perf_metrics 里
// "内存 + 可选 Redis" 的既有形态一致。
const redisStatusKey = "modelprobe:status"

// redisStatusTTL 兜底清理：进程长期不再探测某个模型时让整份状态自然过期，
// 避免 Redis 里留下永不更新的陈旧记录。每轮探测都会刷新它。
const redisStatusTTL = 24 * time.Hour

var (
	memoryMu     sync.RWMutex
	memoryRecord = map[string]Record{}
)

// Save 写入一个模型的探测状态。内存始终写，Redis 可用时同时写，这样即使 Redis
// 临时不可用，本实例的读取也不会退化成空。
func Save(record Record) {
	if record.ModelName == "" {
		return
	}

	memoryMu.Lock()
	memoryRecord[record.ModelName] = record
	memoryMu.Unlock()

	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	payload, err := common.Marshal(record)
	if err != nil {
		common.SysError("model probe: marshal status failed: " + err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pipe := common.RDB.TxPipeline()
	pipe.HSet(ctx, redisStatusKey, record.ModelName, payload)
	pipe.Expire(ctx, redisStatusKey, redisStatusTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		common.SysError("model probe: write status to redis failed: " + err.Error())
	}
}

// LoadAll 读取全部模型状态。Redis 可用时以 Redis 为准；读取失败则退回内存，
// 让状态页降级到"本实例视角"而不是直接空白。
func LoadAll() map[string]Record {
	if common.RedisEnabled && common.RDB != nil {
		if records, ok := loadAllFromRedis(); ok {
			return records
		}
	}

	memoryMu.RLock()
	defer memoryMu.RUnlock()
	records := make(map[string]Record, len(memoryRecord))
	for name, record := range memoryRecord {
		records[name] = record
	}
	return records
}

func loadAllFromRedis() (map[string]Record, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	values, err := common.RDB.HGetAll(ctx, redisStatusKey).Result()
	if err != nil {
		common.SysError("model probe: read status from redis failed: " + err.Error())
		return nil, false
	}
	records := make(map[string]Record, len(values))
	for name, payload := range values {
		var record Record
		if err := common.UnmarshalJsonStr(payload, &record); err != nil {
			common.SysError("model probe: unmarshal status failed for " + name + ": " + err.Error())
			continue
		}
		records[name] = record
	}
	return records, true
}

// Prune 删除不再监测的模型状态，keep 是本轮仍然关心的模型集合。
// 模型下线或被排除后，它的历史状态不应继续出现在状态页上。
func Prune(keep map[string]struct{}) {
	memoryMu.Lock()
	stale := make([]string, 0)
	for name := range memoryRecord {
		if _, ok := keep[name]; !ok {
			stale = append(stale, name)
			delete(memoryRecord, name)
		}
	}
	memoryMu.Unlock()

	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	// Redis 是权威视图，可能含有本实例内存里没有的模型，所以要按 Redis 自己的
	// 键集合判断，不能只删本实例发现的 stale。
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	names, err := common.RDB.HKeys(ctx, redisStatusKey).Result()
	if err != nil {
		common.SysError("model probe: list status keys failed: " + err.Error())
		return
	}
	for _, name := range names {
		if _, ok := keep[name]; !ok {
			stale = append(stale, name)
		}
	}
	if len(stale) == 0 {
		return
	}
	if err := common.RDB.HDel(ctx, redisStatusKey, stale...).Err(); err != nil {
		common.SysError("model probe: prune status failed: " + err.Error())
	}
}

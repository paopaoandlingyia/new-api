package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

const (
	modelRequestConcurrencyNamespace     = "concurrency:model-request:v1"
	modelRequestConcurrencyLeaseDuration = 2 * time.Minute
	modelRequestConcurrencyRenewInterval = 30 * time.Second
	modelRequestConcurrencyRedisTimeout  = 3 * time.Second
)

const redisConcurrencyAcquireScript = `
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now_ms)

local active = redis.call('ZCARD', KEYS[1])
if active >= tonumber(ARGV[1]) then
  return 0
end

redis.call('ZADD', KEYS[1], now_ms + tonumber(ARGV[2]), ARGV[3])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]) * 2)
return 1
`

const redisConcurrencyRenewScript = `
if not redis.call('ZSCORE', KEYS[1], ARGV[2]) then
  return 0
end
local now = redis.call('TIME')
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
redis.call('ZADD', KEYS[1], 'XX', now_ms + tonumber(ARGV[1]), ARGV[2])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[1]) * 2)
return 1
`

const redisConcurrencyReleaseScript = `
local removed = redis.call('ZREM', KEYS[1], ARGV[1])
if redis.call('ZCARD', KEYS[1]) == 0 then
  redis.call('DEL', KEYS[1])
end
return removed
`

type memoryConcurrencyLimiter struct {
	mutex  sync.Mutex
	active map[string]int
}

var modelRequestMemoryConcurrencyLimiter = memoryConcurrencyLimiter{
	active: make(map[string]int),
}

func (l *memoryConcurrencyLimiter) acquire(key string, limit int) (func(), bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.active[key] >= limit {
		return nil, false
	}
	l.active[key]++

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mutex.Lock()
			defer l.mutex.Unlock()
			if l.active[key] <= 1 {
				delete(l.active, key)
				return
			}
			l.active[key]--
		})
	}, true
}

func modelRequestConcurrencyKey(group string, userID int) string {
	return fmt.Sprintf("%s:%d:%s", modelRequestConcurrencyNamespace, userID, group)
}

func newModelRequestConcurrencyLeaseID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func acquireRedisModelRequestConcurrency(ctx context.Context, key string, limit int, leaseID string) (bool, error) {
	if common.RDB == nil {
		return false, errors.New("Redis client is not initialized")
	}

	result, err := common.RDB.Eval(
		ctx,
		redisConcurrencyAcquireScript,
		[]string{key},
		limit,
		modelRequestConcurrencyLeaseDuration.Milliseconds(),
		leaseID,
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func renewRedisModelRequestConcurrency(ctx context.Context, key string, leaseID string) (bool, error) {
	if common.RDB == nil {
		return false, errors.New("Redis client is not initialized")
	}
	result, err := common.RDB.Eval(
		ctx,
		redisConcurrencyRenewScript,
		[]string{key},
		modelRequestConcurrencyLeaseDuration.Milliseconds(),
		leaseID,
	).Int64()
	return result == 1, err
}

func releaseRedisModelRequestConcurrency(ctx context.Context, key string, leaseID string) error {
	if common.RDB == nil {
		return errors.New("Redis client is not initialized")
	}
	return common.RDB.Eval(ctx, redisConcurrencyReleaseScript, []string{key}, leaseID).Err()
}

func keepRedisModelRequestConcurrencyLease(ctx context.Context, key string, leaseID string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(modelRequestConcurrencyRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			renewContext, cancel := context.WithTimeout(context.Background(), modelRequestConcurrencyRedisTimeout)
			renewed, err := renewRedisModelRequestConcurrency(renewContext, key, leaseID)
			cancel()
			if err != nil {
				// A transient Redis failure is observable and the existing lease stays
				// valid until its deadline; later ticks continue trying to renew it.
				logger.LogError(ctx, fmt.Sprintf("model request concurrency lease renewal failed (key=%s): %v", key, err))
				continue
			}
			if !renewed {
				// The lease can disappear only after expiry or external deletion. It
				// cannot be recreated safely because another request may hold the slot.
				logger.LogWarn(ctx, "model request concurrency lease expired before request completion (key=%s)", key)
				return
			}
		case <-stop:
			return
		}
	}
}

func rejectModelRequestConcurrency(c *gin.Context) {
	// The active request may finish at any time, so the lease expiry is not a
	// useful retry estimate. A short hint avoids making clients wait for the
	// full stale-lease recovery window.
	c.Header("Retry-After", "1")
	abortWithOpenAiMessage(c, http.StatusTooManyRequests, "Too many concurrent requests for this group")
}

// ModelRequestConcurrencyLimit limits in-flight relay requests by authenticated
// user and token group. It must run after TokenAuth. An empty group map disables
// the feature without adding another global switch.
func ModelRequestConcurrencyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}
		limit, found := setting.GetModelRequestConcurrencyLimit(group)
		if !found {
			c.Next()
			return
		}

		userID := c.GetInt("id")
		if userID == 0 {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "User authentication is required")
			return
		}
		key := modelRequestConcurrencyKey(group, userID)

		if !common.RedisEnabled {
			release, allowed := modelRequestMemoryConcurrencyLimiter.acquire(key, limit)
			if !allowed {
				rejectModelRequestConcurrency(c)
				return
			}
			defer release()
			c.Next()
			return
		}

		leaseID, err := newModelRequestConcurrencyLeaseID()
		if err != nil {
			logger.LogError(c.Request.Context(), "model request concurrency lease ID generation failed: "+err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "Failed to initialize request concurrency control")
			return
		}
		allowed, err := acquireRedisModelRequestConcurrency(c.Request.Context(), key, limit, leaseID)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("model request concurrency acquisition failed (key=%s): %v", key, err))
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "Failed to check request concurrency")
			return
		}
		if !allowed {
			rejectModelRequestConcurrency(c)
			return
		}

		stopRenewal := make(chan struct{})
		renewalDone := make(chan struct{})
		go keepRedisModelRequestConcurrencyLease(c.Request.Context(), key, leaseID, stopRenewal, renewalDone)
		defer func() {
			close(stopRenewal)
			<-renewalDone
			releaseContext, cancel := context.WithTimeout(context.Background(), modelRequestConcurrencyRedisTimeout)
			defer cancel()
			if err := releaseRedisModelRequestConcurrency(releaseContext, key, leaseID); err != nil {
				// Lease expiry is the recovery path for a process crash or a failed
				// release, and this log keeps that temporary over-limit state visible.
				logger.LogError(c.Request.Context(), fmt.Sprintf("model request concurrency lease release failed (key=%s): %v", key, err))
			}
		}()

		c.Next()
	}
}

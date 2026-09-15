package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useRateLimitMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	previousRedisEnabled := common.RedisEnabled
	previousRedisClient := common.RDB
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	require.NoError(t, redisClient.Ping(context.Background()).Err())

	common.RedisEnabled = true
	common.RDB = redisClient
	t.Cleanup(func() {
		_ = redisClient.Close()
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRedisClient
	})

	return redisServer, redisClient
}

func performRateLimitRequest(router http.Handler, path string, remoteAddr string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = remoteAddr
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRedisIPRateLimiterThresholdTTLAndNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, _ := useRateLimitMiniRedis(t)

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/limited", rateLimitFactory(2, 37, "TEST"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	remoteAddr := "192.0.2.10:12345"
	legacyKey := "rateLimit:TEST192.0.2.10"
	_, err := redisServer.Push(legacyKey, "legacy-list-entry")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/limited", remoteAddr).Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/limited", remoteAddr).Code)
	limitedResponse := performRateLimitRequest(router, "/limited", remoteAddr)
	assert.Equal(t, http.StatusTooManyRequests, limitedResponse.Code)
	assert.Equal(t, "37", limitedResponse.Header().Get("Retry-After"))

	key := redisIPRateLimitKey("TEST", "192.0.2.10")
	count, err := redisServer.Get(key)
	require.NoError(t, err)
	assert.Equal(t, "3", count)
	assert.Equal(t, 37*time.Second, redisServer.TTL(key))
	assert.True(t, redisServer.Exists(legacyKey), "the v2 counter must not touch an old list key")
}

func TestRedisUserRateLimiterUsesSharedFixedWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, _ := useRateLimitMiniRedis(t)

	router := gin.New()
	router.GET(
		"/limited",
		func(c *gin.Context) { c.Set("id", 42) },
		userRateLimitFactory(1, 23, "USER"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/limited", "192.0.2.20:12345").Code)
	assert.Equal(t, http.StatusTooManyRequests, performRateLimitRequest(router, "/limited", "198.51.100.20:12345").Code)

	key := redisUserRateLimitKey("USER", 42)
	assert.True(t, redisServer.Exists(key))
	assert.Equal(t, 23*time.Second, redisServer.TTL(key))
}

func TestRedisEmailVerificationRateLimiterPreservesResponseAndTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, _ := useRateLimitMiniRedis(t)

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/verify", EmailVerificationRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	remoteAddr := "192.0.2.30:12345"
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/verify", remoteAddr).Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/verify", remoteAddr).Code)
	response := performRateLimitRequest(router, "/verify", remoteAddr)
	assert.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.JSONEq(t, `{"success":false,"message":"发送过于频繁，请等待 30 秒后再试"}`, response.Body.String())

	key := redisIPRateLimitKey(EmailVerificationRateLimitMark, "192.0.2.30")
	assert.True(t, redisServer.Exists(key))
	assert.Equal(t, time.Duration(EmailVerificationDuration)*time.Second, redisServer.TTL(key))
}

func TestRedisFixedWindowIsAtomicUnderConcurrency(t *testing.T) {
	redisServer, _ := useRateLimitMiniRedis(t)
	const (
		requestCount = 20
		maximumCount = 7
		duration     = int64(41)
	)
	key := redisIPRateLimitKey("CONCURRENT", "192.0.2.40")

	var allowedCount atomic.Int64
	errorsFound := make(chan error, requestCount)
	var waitGroup sync.WaitGroup
	waitGroup.Add(requestCount)
	for range requestCount {
		go func() {
			defer waitGroup.Done()
			allowed, _, _, err := redisFixedWindowTake(context.Background(), key, maximumCount, duration)
			if err != nil {
				errorsFound <- err
				return
			}
			if allowed {
				allowedCount.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	close(errorsFound)
	for err := range errorsFound {
		require.NoError(t, err)
	}

	assert.Equal(t, int64(maximumCount), allowedCount.Load())
	count, err := redisServer.Get(key)
	require.NoError(t, err)
	assert.Equal(t, "20", count)
	assert.Equal(t, time.Duration(duration)*time.Second, redisServer.TTL(key))
}

func TestRedisFixedWindowResetsAtBoundary(t *testing.T) {
	redisServer, _ := useRateLimitMiniRedis(t)
	const duration = int64(10)
	key := redisIPRateLimitKey("BOUNDARY", "192.0.2.50")

	for range 2 {
		allowed, _, _, err := redisFixedWindowTake(context.Background(), key, 2, duration)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, _, _, err := redisFixedWindowTake(context.Background(), key, 2, duration)
	require.NoError(t, err)
	assert.False(t, allowed)

	// This reset is intentional fixed-window behavior. A client can consume one
	// full allowance immediately before and another immediately after a boundary.
	redisServer.FastForward(time.Duration(duration) * time.Second)
	for range 2 {
		allowed, _, _, err = redisFixedWindowTake(context.Background(), key, 2, duration)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
}

func TestRedisFixedWindowRepairsCounterWithoutTTL(t *testing.T) {
	redisServer, _ := useRateLimitMiniRedis(t)
	const duration = int64(29)
	key := redisIPRateLimitKey("MISSING-TTL", "192.0.2.51")
	redisServer.Set(key, "5")

	allowed, count, ttl, err := redisFixedWindowTake(context.Background(), key, 3, duration)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, int64(6), count)
	assert.Equal(t, duration, ttl)
	assert.Equal(t, time.Duration(duration)*time.Second, redisServer.TTL(key))

	redisServer.FastForward(time.Duration(duration) * time.Second)
	assert.False(t, redisServer.Exists(key), "a recovered counter must not remain permanently rate-limited")
}

func TestRedisFailurePolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, redisClient := useRateLimitMiniRedis(t)
	require.NoError(t, redisClient.Close())

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/ip", rateLimitFactory(1, 30, "FAIL-IP"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET(
		"/user",
		func(c *gin.Context) { c.Set("id", 7) },
		userRateLimitFactory(1, 30, "FAIL-USER"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)
	router.GET("/email", EmailVerificationRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	ipResponse := performRateLimitRequest(router, "/ip", "192.0.2.60:12345")
	assert.Equal(t, http.StatusInternalServerError, ipResponse.Code)
	assert.Empty(t, ipResponse.Body.String())
	userResponse := performRateLimitRequest(router, "/user", "192.0.2.61:12345")
	assert.Equal(t, http.StatusInternalServerError, userResponse.Code)
	assert.Empty(t, userResponse.Body.String())
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/email", "192.0.2.62:12345").Code)
}

func useModelRequestConcurrencyLimits(t *testing.T, limits string) {
	t.Helper()

	previousLimits := setting.ModelRequestConcurrencyLimitGroup2JSONString()
	require.NoError(t, setting.UpdateModelRequestConcurrencyLimitGroupByJSONString(limits))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateModelRequestConcurrencyLimitGroupByJSONString(previousLimits))
	})
}

func resetMemoryModelRequestConcurrencyLimiter(t *testing.T) {
	t.Helper()

	modelRequestMemoryConcurrencyLimiter.mutex.Lock()
	previousActive := modelRequestMemoryConcurrencyLimiter.active
	modelRequestMemoryConcurrencyLimiter.active = make(map[string]int)
	modelRequestMemoryConcurrencyLimiter.mutex.Unlock()
	t.Cleanup(func() {
		modelRequestMemoryConcurrencyLimiter.mutex.Lock()
		modelRequestMemoryConcurrencyLimiter.active = previousActive
		modelRequestMemoryConcurrencyLimiter.mutex.Unlock()
	})
}

func modelRequestConcurrencyRouter(started chan<- struct{}, release <-chan struct{}) *gin.Engine {
	router := gin.New()
	router.GET(
		"/relay",
		func(c *gin.Context) {
			userID, err := strconv.Atoi(c.Query("user"))
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			c.Set("id", userID)
			common.SetContextKey(c, constant.ContextKeyTokenGroup, c.Query("group"))
		},
		ModelRequestConcurrencyLimit(),
		func(c *gin.Context) {
			if c.Query("hold") == "true" {
				started <- struct{}{}
				<-release
			}
			c.Status(http.StatusNoContent)
		},
	)
	return router
}

func TestMemoryModelRequestConcurrencyLimitScopesAndReleases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useModelRequestConcurrencyLimits(t, `{"cheap":1,"other":1}`)
	resetMemoryModelRequestConcurrencyLimiter(t)

	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	router := modelRequestConcurrencyRouter(started, release)
	firstResponse := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstResponse <- performRateLimitRequest(router, "/relay?user=1&group=cheap&hold=true", "192.0.2.70:12345")
	}()
	<-started

	limited := performRateLimitRequest(router, "/relay?user=1&group=cheap", "192.0.2.71:12345")
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "1", limited.Header().Get("Retry-After"))
	assert.Contains(t, limited.Body.String(), "Too many concurrent requests for this group")
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/relay?user=2&group=cheap", "192.0.2.72:12345").Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/relay?user=1&group=other", "192.0.2.73:12345").Code)

	close(release)
	assert.Equal(t, http.StatusNoContent, (<-firstResponse).Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/relay?user=1&group=cheap", "192.0.2.74:12345").Code)
}

func TestRedisModelRequestConcurrencyLimitSharesAndReleasesLease(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, redisClient := useRateLimitMiniRedis(t)
	useModelRequestConcurrencyLimits(t, `{"cheap":1}`)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	router := modelRequestConcurrencyRouter(started, release)
	firstResponse := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstResponse <- performRateLimitRequest(router, "/relay?user=7&group=cheap&hold=true", "192.0.2.80:12345")
	}()
	<-started

	key := modelRequestConcurrencyKey("cheap", 7)
	active, err := redisClient.ZCard(context.Background(), key).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), active)
	limited := performRateLimitRequest(router, "/relay?user=7&group=cheap", "192.0.2.81:12345")
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "1", limited.Header().Get("Retry-After"))

	close(release)
	assert.Equal(t, http.StatusNoContent, (<-firstResponse).Code)
	assert.False(t, redisServer.Exists(key))
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/relay?user=7&group=cheap", "192.0.2.82:12345").Code)
}

func TestModelRequestConcurrencyLimitConfigurationValidation(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty object disables limits", value: `{}`},
		{name: "positive group limit", value: `{"cheap":1}`},
		{name: "null is not an object", value: `null`, wantErr: true},
		{name: "empty group", value: `{"":1}`, wantErr: true},
		{name: "zero limit", value: `{"cheap":0}`, wantErr: true},
		{name: "negative limit", value: `{"cheap":-1}`, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := setting.CheckModelRequestConcurrencyLimitGroup(test.value)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

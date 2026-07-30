package model

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConsumeLogsFilterAndSumByUpstreamAccount(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousLogDatabaseType := common.LogDatabaseType()
	previousLogConsumeEnabled := common.LogConsumeEnabled
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetLogDatabaseType(previousLogDatabaseType)
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})

	dsn := fmt.Sprintf("file:log-upstream-account-%s?mode=memory&cache=shared", common.GetRandomString(8))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &User{}))
	DB, LOG_DB = db, db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	common.LogConsumeEnabled = true
	initCol()

	gin.SetMode(gin.TestMode)
	for account, quota := range map[string]int{"friend-a": 120, "friend-b": 80} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		ctx.Set(common.UpstreamAccountKey, account)
		RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
			ModelName:        "claude-sonnet-4-5",
			Quota:            quota,
			PromptTokens:     10,
			CompletionTokens: 5,
		})
	}

	logs, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 20, 0, "", "", "", "friend-a")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	assert.Equal(t, "friend-a", logs[0].UpstreamAccount)
	assert.Equal(t, 120, logs[0].Quota)

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", 0, "", "friend-a")
	require.NoError(t, err)
	assert.Equal(t, 120, stat.Quota)
	assert.Equal(t, 1, stat.Rpm)
	assert.Equal(t, 15, stat.Tpm)

	userLogs, _, err := GetUserLogs(1, LogTypeConsume, 0, 0, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.Len(t, userLogs, 2)
	assert.Empty(t, userLogs[0].UpstreamAccount)
	assert.Empty(t, userLogs[1].UpstreamAccount)
}

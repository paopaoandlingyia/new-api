package model

import (
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClaudeRefusalLogFilterDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres", "clickhouse"} {
		t.Run(dialect, func(t *testing.T) {
			var db *gorm.DB
			var err error
			if dialect == "sqlite" {
				db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			} else {
				dsn := os.Getenv("TEST_" + map[string]string{"mysql": "MYSQL", "postgres": "POSTGRES", "clickhouse": "CLICKHOUSE"}[dialect] + "_DSN")
				if dsn == "" {
					t.Skip("test database DSN is not configured")
				}
				t.Setenv("LOG_SQL_DSN", dsn)
				db, _, err = chooseDB("LOG_SQL_DSN", true)
			}
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB, previousLogDB := DB, LOG_DB
			previousLogType := common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetLogDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(&Log{}))
				DB, LOG_DB = previousDB, previousLogDB
				common.SetLogDatabaseType(previousLogType)
				initCol()
				require.NoError(t, sqlDB.Close())
			})
			if dialect == "clickhouse" {
				require.NoError(t, db.Exec(clickHouseLogCreateTableSQL(0)).Error)
			} else {
				require.NoError(t, db.AutoMigrate(&Log{}))
			}
			var version string
			versionQuery := "SELECT version()"
			if dialect == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database version: %s", version)

			now := time.Now().Unix()
			rows := []Log{
				{RequestId: "refusal-new", CreatedAt: now, Quota: 20, PromptTokens: 7, CompletionTokens: 3, Other: `{"admin_info":{"reject_reason":"claude_stop_reason=refusal","refusal_category":"cyber"}}`},
				{RequestId: "refusal-legacy", CreatedAt: now - 120, Quota: 30, Other: `{ "admin_info" : { "reject_reason" : "claude_stop_reason=refusal" } }`},
				{RequestId: "normal", CreatedAt: now, Quota: 40, Other: `{}`},
				{RequestId: "empty", CreatedAt: now, Quota: 50},
				{RequestId: "different-rejection", CreatedAt: now, Quota: 60, Other: `{"admin_info":{"reject_reason":"ingress_status=403"}}`},
				{RequestId: "text-lookalike", CreatedAt: now, Quota: 70, Other: `{"description":"claude_stop_reason=refusal","reject_reason":"claude_stop_reason=refusal"}`},
				{RequestId: "prefix-lookalike", CreatedAt: now, Quota: 80, Other: `{"admin_info":{"reject_reason":"claude_stop_reason=refusal_extra"}}`},
				{RequestId: "case-lookalike", CreatedAt: now, Quota: 90, Other: `{"admin_info":{"reject_reason":"CLAUDE_STOP_REASON=REFUSAL"}}`},
				{RequestId: "null", CreatedAt: now, Quota: 100, Other: `{"admin_info":{"reject_reason":null}}`},
			}
			for i := range rows {
				rows[i].Type = LogTypeConsume
				rows[i].UserId = 1
				rows[i].Username = "merchant"
				rows[i].UpstreamAccount = "friend-a"
				rows[i].ModelName = "claude"
			}
			require.NoError(t, db.Create(&rows).Error)
			for page, requestID := range []string{"refusal-new", "refusal-legacy"} {
				logs, total, err := GetAllLogs(LogTypeUnknown, now-300, now+1, "claude", "merchant", "", page, 1, 0, "", "", "", "friend-a", true)
				require.NoError(t, err)
				assert.Equal(t, int64(2), total)
				require.Len(t, logs, 1)
				assert.Equal(t, requestID, logs[0].RequestId)
			}
			logs, total, err := GetAllLogs(LogTypeConsume, now-300, now+1, "", "", "", 2, 1, 0, "", "", "", "", true)
			require.NoError(t, err)
			assert.Equal(t, int64(2), total)
			assert.Empty(t, logs)
			_, total, err = GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 20, 0, "", "", "", "", false)
			require.NoError(t, err)
			assert.Equal(t, int64(len(rows)), total)
			_, total, err = GetAllLogs(LogTypeConsume, now-60, now+1, "", "", "", 0, 20, 0, "", "", "", "", true)
			require.NoError(t, err)
			assert.Equal(t, int64(1), total)
			_, total, err = GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 20, 0, "", "normal", "", "", true)
			require.NoError(t, err)
			assert.Zero(t, total)
			stat, err := SumUsedQuota(LogTypeConsume, now-300, now+1, "claude", "merchant", "", 0, "", "friend-a", true)
			require.NoError(t, err)
			assert.Equal(t, Stat{Quota: 50, Rpm: 1, Tpm: 10}, stat)
			stat, err = SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", 0, "", "missing-account", true)
			require.NoError(t, err)
			assert.Equal(t, Stat{}, stat)
		})
	}
}

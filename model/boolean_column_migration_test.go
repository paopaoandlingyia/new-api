package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureUnmanagedColumnsAddsMissingColumns(t *testing.T) {
	previousDB := DB
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(&CustomOAuthProvider{}, &SubscriptionPlan{}))
	require.False(t, db.Migrator().HasColumn(&CustomOAuthProvider{}, "enabled"))
	require.False(t, db.Migrator().HasColumn(&SubscriptionPlan{}, "enabled"))
	require.False(t, db.Migrator().HasColumn(&SubscriptionPlan{}, "price_amount"))

	require.NoError(t, ensureUnmanagedColumns())
	require.True(t, db.Migrator().HasColumn(&CustomOAuthProvider{}, "enabled"))
	require.True(t, db.Migrator().HasColumn(&SubscriptionPlan{}, "enabled"))
	require.True(t, db.Migrator().HasColumn(&SubscriptionPlan{}, "price_amount"))
	require.NoError(t, ensureUnmanagedColumns())
}

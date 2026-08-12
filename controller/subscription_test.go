package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRejectBalanceOnlySubscription(t *testing.T) {
	tests := []struct {
		name       string
		plan       *model.SubscriptionPlan
		wantReject bool
	}{
		{name: "regular plan allows external payment", plan: &model.SubscriptionPlan{}, wantReject: false},
		{name: "balance-only plan rejects external payment", plan: &model.SubscriptionPlan{BalanceOnly: true}, wantReject: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)

			rejected := rejectBalanceOnlySubscription(ctx, test.plan)

			require.Equal(t, test.wantReject, rejected)
			if !test.wantReject {
				require.Equal(t, http.StatusOK, recorder.Code)
				return
			}
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Equal(t, "该套餐仅支持余额购买", response.Message)
		})
	}
}

func TestAdminCreateSubscriptionPlanEnabledDefault(t *testing.T) {
	confirmPaymentComplianceForTest(t)

	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}))
	require.NoError(t, db.Exec("ALTER TABLE subscription_plans ADD COLUMN enabled numeric").Error)
	require.NoError(t, db.Exec("ALTER TABLE subscription_plans ADD COLUMN price_amount numeric").Error)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	tests := []struct {
		name        string
		title       string
		body        string
		wantEnabled bool
	}{
		{
			name:        "omitted enabled defaults to true",
			title:       "Default plan",
			body:        `{"plan":{"title":"Default plan"}}`,
			wantEnabled: true,
		},
		{
			name:        "explicit false is preserved",
			title:       "Disabled plan",
			body:        `{"plan":{"title":"Disabled plan","enabled":false}}`,
			wantEnabled: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/plans", bytes.NewBufferString(test.body))
			ctx.Request.Header.Set("Content-Type", "application/json")

			AdminCreateSubscriptionPlan(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			var plan model.SubscriptionPlan
			require.NoError(t, db.Where("title = ?", test.title).First(&plan).Error)
			require.Equal(t, test.wantEnabled, plan.Enabled)
		})
	}
}

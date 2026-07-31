package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdminCreateSubscriptionPlanEnabledDefault(t *testing.T) {
	confirmPaymentComplianceForTest(t)

	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}))
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

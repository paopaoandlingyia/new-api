package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingOnlyExposesSelectableGroupsAcrossCustomers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := modelManagementDB(t, "sqlite", "")
	previousUsable := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	previousSpecial := special.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsable))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		special.Clear()
		special.AddAll(previousSpecial)
		model.InvalidatePricingCache()
	})
	special.Clear()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"public":"Public","auto":"Automatic","unconfigured":"Missing configuration"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"public":1,"private-a":2,"private-b":3}`))
	require.NoError(t, db.Create(&[]model.User{
		{Id: 201, Username: "customer-a", AffCode: "pricing-a", Group: "private-a", Status: common.UserStatusEnabled},
		{Id: 202, Username: "customer-b", AffCode: "pricing-b", Group: "private-b", Status: common.UserStatusEnabled},
	}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "isolated", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "private-a", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "private-b", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "private-b", Model: "b-only-model", ChannelId: 1, Enabled: true},
		{Group: "public", Model: "public-model", ChannelId: 1, Enabled: true},
		{Group: "all", Model: "universal-model", ChannelId: 1, Enabled: true},
	}).Error)
	model.InvalidatePricingCache()

	for _, tc := range []struct {
		name       string
		userID     int
		group      string
		wantModels int
	}{
		{"customer a", 201, "private-a", 3},
		{"customer b using the same cache", 202, "private-b", 4},
		{"customer a after customer b", 201, "private-a", 3},
		{"anonymous visitor", 0, "", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
			if tc.userID != 0 {
				c.Set("id", tc.userID)
			}
			GetPricing(c)
			var response struct {
				Success     bool               `json:"success"`
				Data        []model.Pricing    `json:"data"`
				UsableGroup map[string]string  `json:"usable_group"`
				GroupRatio  map[string]float64 `json:"group_ratio"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Len(t, response.Data, tc.wantModels)
			expectedUsable := map[string]string{"public": "Public", "auto": "Automatic"}
			expectedRatios := map[string]float64{"public": 1}
			expectedAll := []string{"public"}
			if tc.group != "" {
				expectedUsable[tc.group] = "User group"
				expectedRatios[tc.group] = map[string]float64{"private-a": 2, "private-b": 3}[tc.group]
				expectedAll = append(expectedAll, tc.group)
			}
			assert.Equal(t, expectedUsable, response.UsableGroup)
			assert.Equal(t, expectedRatios, response.GroupRatio)
			byName := pricingByModelName(response.Data)
			require.Contains(t, byName, "public-model")
			assert.Equal(t, []string{"public"}, byName["public-model"].EnableGroup)
			require.Contains(t, byName, "universal-model")
			assert.ElementsMatch(t, expectedAll, byName["universal-model"].EnableGroup)
			if tc.group != "" {
				require.Contains(t, byName, "shared-model")
				assert.Equal(t, []string{tc.group}, byName["shared-model"].EnableGroup)
			} else {
				assert.NotContains(t, byName, "shared-model")
			}
			if tc.group == "private-b" {
				assert.Contains(t, byName, "b-only-model")
			} else {
				assert.NotContains(t, byName, "b-only-model")
			}
		})
	}
	cached := pricingByModelName(model.GetPricing())
	assert.ElementsMatch(t, []string{"private-a", "private-b"}, cached["shared-model"].EnableGroup)
	assert.Equal(t, []string{"all"}, cached["universal-model"].EnableGroup)
}

func TestFilterPricingWithNoSelectableGroups(t *testing.T) {
	for _, groups := range []map[string]string{nil, {}, {"auto": "Automatic"}} {
		got := filterPricingByUsableGroups([]model.Pricing{
			{ModelName: "private", EnableGroup: []string{"private-a"}},
			{ModelName: "universal", EnableGroup: []string{"all"}},
			{ModelName: "empty"},
		}, groups)
		assert.Empty(t, got)
	}
	assert.Nil(t, filterPricingByUsableGroups(nil, map[string]string{"public": "Public"}))
}

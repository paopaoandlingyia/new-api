package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A plan with max_active_per_user set caps how many of its subscriptions one user may
// hold at the same time, which is what keeps the plan's quota rate (e.g. 75 per 7 days)
// from being multiplied by buying several copies in the same period.
func TestCheckSubscriptionActiveLimit(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9301,
		Title:            "Weekly",
		PriceAmount:      50,
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    7,
		TotalAmount:      75000,
		QuotaResetPeriod: SubscriptionResetNever,
		MaxActivePerUser: 1,
	}
	require.NoError(t, DB.Create(plan).Error)

	otherPlan := &SubscriptionPlan{
		Id:               9302,
		Title:            "Monthly",
		PriceAmount:      100,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      300000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(otherPlan).Error)

	seed := func(id int, userId int, planId int, endTime int64, status string) {
		require.NoError(t, DB.Create(&UserSubscription{
			Id: id, UserId: userId, PlanId: planId,
			AmountTotal: 75000, StartTime: now - 3600, EndTime: endTime, Status: status,
		}).Error)
	}

	// User 501 holds an expired instance and a cancelled one: neither is active.
	seed(9401, 501, plan.Id, now-1, "active")
	seed(9402, 501, plan.Id, now+7*24*3600, "cancelled")
	// User 502 holds a live instance of the capped plan.
	seed(9403, 502, plan.Id, now+7*24*3600, "active")
	// User 503 holds a live instance of a different plan only.
	seed(9404, 503, otherPlan.Id, now+30*24*3600, "active")

	require.NoError(t, CheckSubscriptionActiveLimit(501, plan))
	require.ErrorIs(t, CheckSubscriptionActiveLimit(502, plan), ErrSubscriptionActiveLimitReached)
	require.NoError(t, CheckSubscriptionActiveLimit(503, plan))

	// A plan without the cap keeps the legacy stacking behavior.
	require.NoError(t, CheckSubscriptionActiveLimit(503, otherPlan))

	// The cap is a count, not a boolean.
	plan.MaxActivePerUser = 2
	require.NoError(t, CheckSubscriptionActiveLimit(502, plan))

	count, err := countActiveUserSubscriptionsByPlanTx(nil, 502, plan.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

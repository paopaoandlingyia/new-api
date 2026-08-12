package model

import (
	"errors"

	"gorm.io/gorm"
)

// ErrSubscriptionActiveLimitReached is returned when a plan caps how many of its
// subscriptions a single user may have active at the same time.
var ErrSubscriptionActiveLimitReached = errors.New("该套餐已有生效中的订阅，到期后才能再次购买")

// countActiveUserSubscriptionsByPlanTx counts a user's currently active subscriptions of one plan.
func countActiveUserSubscriptionsByPlanTx(tx *gorm.DB, userId int, planId int) (int64, error) {
	if tx == nil {
		tx = DB
	}
	var count int64
	err := tx.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ? AND status = ? AND end_time > ?",
			userId, planId, "active", GetDBTimestamp()).
		Count(&count).Error
	return count, err
}

// EnforceSubscriptionActiveLimitTx takes a row lock on the user and rejects the purchase
// when the plan's concurrent-active cap is already reached.
//
// The lock serializes every subscription creation path for one user (payment callback,
// admin grant, balance redeem), so the plan's lifetime purchase cap checked right after
// this call is protected from concurrent duplicate callbacks as well.
func EnforceSubscriptionActiveLimitTx(tx *gorm.DB, userId int, plan *SubscriptionPlan) error {
	if tx == nil || plan == nil || plan.Id <= 0 || userId <= 0 {
		return errors.New("invalid subscription active limit args")
	}
	if _, err := getUserGroupByIdTx(tx, userId); err != nil {
		return err
	}
	if plan.MaxActivePerUser <= 0 {
		return nil
	}
	count, err := countActiveUserSubscriptionsByPlanTx(tx, userId, plan.Id)
	if err != nil {
		return err
	}
	if count >= int64(plan.MaxActivePerUser) {
		return ErrSubscriptionActiveLimitReached
	}
	return nil
}

// CheckSubscriptionActiveLimit is the pre-payment check, so a user is rejected before
// paying instead of at order completion. The authoritative check stays in
// EnforceSubscriptionActiveLimitTx.
func CheckSubscriptionActiveLimit(userId int, plan *SubscriptionPlan) error {
	if plan == nil || plan.MaxActivePerUser <= 0 {
		return nil
	}
	count, err := countActiveUserSubscriptionsByPlanTx(nil, userId, plan.Id)
	if err != nil {
		return err
	}
	if count >= int64(plan.MaxActivePerUser) {
		return ErrSubscriptionActiveLimitReached
	}
	return nil
}

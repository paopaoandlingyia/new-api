package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cryptopay"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const PaymentProviderCrypto = "crypto"

// CryptoWallet is also the allocation lock and durable scanner checkpoint.
// Addresses remain monitored after configuration changes so old orders settle.
type CryptoWallet struct {
	ID                int64  `json:"id" gorm:"primaryKey"`
	Network           string `json:"network" gorm:"type:varchar(32);uniqueIndex:crypto_wallet"`
	Asset             string `json:"asset" gorm:"type:varchar(32);uniqueIndex:crypto_wallet"`
	Address           string `json:"address" gorm:"type:varchar(64);uniqueIndex:crypto_wallet"`
	CreatedAt         int64  `json:"created_at"`
	AllocationVersion int64  `json:"-"`
	ScanFrom          int64  `json:"scan_from"`
	ScanTo            int64  `json:"scan_to"`
	ScanCursor        string `json:"-" gorm:"type:text"`
	ScannedThrough    int64  `json:"scanned_through"`
	LastFullScan      int64  `json:"last_full_scan"`
}

// AmountUnits is integer micro-USDT. The unique reservation is permanent:
// arbitrary late payments make finite-time amount reuse inherently ambiguous.
// Each cent of base price has 9999 distinct suffixes below one additional cent.
type CryptoPaymentOrder struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	TradeNo     string `json:"trade_no" gorm:"type:varchar(255);uniqueIndex"`
	UserID      int    `json:"user_id" gorm:"index"`
	WalletID    int64  `json:"wallet_id" gorm:"uniqueIndex:crypto_amount_reservation"`
	AmountUnits int64  `json:"-" gorm:"uniqueIndex:crypto_amount_reservation"`
	CreditQuota int64  `json:"credit_quota"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at" gorm:"index"`
}

type CryptoTransfer struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	Network        string `json:"network" gorm:"type:varchar(32);uniqueIndex:crypto_transfer_event"`
	Asset          string `json:"asset" gorm:"type:varchar(32)"`
	TransactionID  string `json:"transaction_id" gorm:"type:varchar(128);uniqueIndex:crypto_transfer_event"`
	EventIndex     int    `json:"event_index" gorm:"uniqueIndex:crypto_transfer_event"`
	Address        string `json:"address" gorm:"type:varchar(64)"`
	AmountUnits    int64  `json:"amount_units"`
	BlockTimestamp int64  `json:"block_timestamp"`
	TradeNo        string `json:"trade_no" gorm:"type:varchar(255);index"`
	ReviewReason   string `json:"review_reason" gorm:"type:varchar(255)"`
}

func EnsureCryptoWallet(network, asset, address string) (*CryptoWallet, error) {
	wallet := CryptoWallet{Network: network, Asset: asset, Address: address, CreatedAt: time.Now().UnixMilli()}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&wallet).Error; err != nil {
		return nil, err
	}
	if err := DB.Where("network = ? AND asset = ? AND address = ?", network, asset, address).First(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

func CreateCryptoPaymentOrder(walletID int64, topup *TopUp, creditQuota int, baseUnits int64, minutes int64) (*CryptoPaymentOrder, error) {
	if creditQuota <= 0 || creditQuota > common.MaxWalletQuota || baseUnits <= 0 || baseUnits%10000 != 0 || baseUnits > 1000000000000 || minutes < 5 || minutes > 1440 {
		return nil, fmt.Errorf("invalid crypto payment amount or window")
	}
	order := &CryptoPaymentOrder{TradeNo: topup.TradeNo, UserID: topup.UserId, WalletID: walletID, CreditQuota: int64(creditQuota), CreatedAt: time.Now().UnixMilli()}
	order.ExpiresAt = order.CreatedAt + minutes*60000
	err := DB.Transaction(func(tx *gorm.DB) error {
		// This write also serializes SQLite allocation transactions, where row
		// locks are unavailable. Never rely on a process-local allocation mutex.
		result := tx.Model(&CryptoWallet{}).Where("id = ?", walletID).Update("allocation_version", gorm.Expr("allocation_version + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		var highest CryptoPaymentOrder
		err := tx.Where("wallet_id = ? AND amount_units > ? AND amount_units < ?", walletID, baseUnits, baseUnits+10000).Order("amount_units desc").First(&highest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		order.AmountUnits = max(baseUnits+1, highest.AmountUnits+1)
		if order.AmountUnits >= baseUnits+10000 {
			return fmt.Errorf("payment amounts for this price are exhausted; change the price or receiving address")
		}
		topup.Money = decimal.NewFromInt(order.AmountUnits).Shift(-6).InexactFloat64()
		if err := tx.Create(topup).Error; err != nil {
			return err
		}
		return tx.Create(order).Error
	})
	return order, err
}

// RecordCryptoTransfer is idempotent across page replay and multiple masters.
func RecordCryptoTransfer(transfer cryptopay.Transfer) (*CryptoTransfer, bool, error) {
	if transfer.Amount <= 0 || transfer.Timestamp <= 0 || transfer.TransactionID == "" || transfer.EventIndex < 0 {
		return nil, false, fmt.Errorf("invalid confirmed transfer")
	}
	record := &CryptoTransfer{Network: transfer.Network, Asset: transfer.Asset, TransactionID: transfer.TransactionID, EventIndex: transfer.EventIndex, Address: transfer.Address, AmountUnits: transfer.Amount, BlockTimestamp: transfer.Timestamp}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return nil, false, result.Error
	}
	inserted := result.RowsAffected == 1
	if err := DB.Where("network = ? AND transaction_id = ? AND event_index = ?", transfer.Network, transfer.TransactionID, transfer.EventIndex).First(record).Error; err != nil {
		return nil, false, err
	}
	if record.Asset != transfer.Asset || record.Address != transfer.Address || record.AmountUnits != transfer.Amount || record.BlockTimestamp != transfer.Timestamp {
		return nil, false, fmt.Errorf("conflicting confirmed transfer %s/%d", transfer.TransactionID, transfer.EventIndex)
	}
	return record, inserted, nil
}

// SettleCryptoTransfer atomically consumes the receipt, completes the order,
// and credits the quota snapshot. Manual reconciliation may accept a late or
// differently sized payment, but always requires an unused confirmed receipt.
func SettleCryptoTransfer(transferID int64, manualTradeNo string, callerIP string) (bool, error) {
	var credited bool
	var userID, quota int
	var tradeNo string
	var review string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var transfer CryptoTransfer
		if err := lockForUpdate(tx).First(&transfer, transferID).Error; err != nil {
			return err
		}
		if transfer.TradeNo != "" {
			if manualTradeNo != "" && transfer.TradeNo != manualTradeNo {
				return fmt.Errorf("transfer already belongs to another order")
			}
			return nil
		}
		var wallet CryptoWallet
		if err := tx.Where("network = ? AND asset = ? AND address = ?", transfer.Network, transfer.Asset, transfer.Address).First(&wallet).Error; err != nil {
			return err
		}
		var order CryptoPaymentOrder
		query := tx.Where("wallet_id = ?", wallet.ID)
		if manualTradeNo != "" {
			query = query.Where("trade_no = ?", manualTradeNo)
		} else {
			query = query.Where("amount_units = ?", transfer.AmountUnits)
		}
		err := query.First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) && manualTradeNo == "" {
			review = "unmatched_amount"
		} else if err != nil {
			return err
		}
		if review == "" && manualTradeNo == "" && (transfer.BlockTimestamp < order.CreatedAt || transfer.BlockTimestamp > order.ExpiresAt) {
			review = "outside_payment_window"
		}
		if review != "" {
			return tx.Model(&transfer).Update("review_reason", review).Error
		}
		var topup TopUp
		if err := lockForUpdate(tx).Where("trade_no = ? AND payment_provider = ?", order.TradeNo, PaymentProviderCrypto).First(&topup).Error; err != nil {
			return err
		}
		if topup.Status == common.TopUpStatusSuccess {
			review = "order_already_paid"
			if manualTradeNo != "" {
				return fmt.Errorf("order already paid")
			}
			return tx.Model(&transfer).Update("review_reason", review).Error
		}
		if topup.Status != common.TopUpStatusPending && topup.Status != common.TopUpStatusExpired {
			return ErrTopUpStatusInvalid
		}
		quota, err = common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(order.CreditQuota))
		if err != nil || quota <= 0 {
			return ErrInvalidTopUpQuota
		}
		if err := creditTopUpQuota(tx, topup.UserId, quota, nil); err != nil {
			return err
		}
		result := tx.Model(&topup).Where("status IN ?", []string{common.TopUpStatusPending, common.TopUpStatusExpired}).Updates(map[string]any{"status": common.TopUpStatusSuccess, "complete_time": common.GetTimestamp()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("crypto order changed during settlement")
		}
		result = tx.Model(&transfer).Where("trade_no = ?", "").Updates(map[string]any{"trade_no": order.TradeNo, "review_reason": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("crypto transfer changed during settlement")
		}
		userID, tradeNo, credited = topup.UserId, order.TradeNo, true
		return nil
	})
	if err != nil {
		return false, err
	}
	if credited {
		syncCreditUserQuotaCache(userID, quota, "crypto topup")
		method := "chain"
		if manualTradeNo != "" {
			method = "admin"
		}
		RecordTopupLog(userID, fmt.Sprintf("USDT充值成功，订单: %s，充值额度: %d，到账记录: %d", tradeNo, quota, transferID), callerIP, PaymentProviderCrypto, method)
	}
	return credited, nil
}

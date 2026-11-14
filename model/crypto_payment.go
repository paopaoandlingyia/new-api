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

const (
	PaymentProviderCrypto    = "crypto"
	cryptoPaymentAmountStep  = int64(1000)   // 0.001 USDT, stored as micro-USDT.
	cryptoPaymentAmountRange = int64(100000) // Identifying suffix below 0.1 USDT.
	cryptoPaymentCooldown    = 24 * time.Hour
)

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

// AmountUnits is integer micro-USDT, including legacy six-decimal orders.
// Wallet allocation is serialized; an amount can recur in non-overlapping
// payment windows after its previous uses have cooled down.
type CryptoPaymentOrder struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	TradeNo     string `json:"trade_no" gorm:"type:varchar(255);uniqueIndex"`
	UserID      int    `json:"user_id" gorm:"index"`
	WalletID    int64  `json:"wallet_id" gorm:"index:crypto_order_amount"`
	AmountUnits int64  `json:"-" gorm:"index:crypto_order_amount"`
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
	RawAmount      string `json:"raw_amount,omitempty" gorm:"type:varchar(78)"`
	BlockTimestamp int64  `json:"block_timestamp"`
	TradeNo        string `json:"trade_no" gorm:"type:varchar(255);index"`
	ReviewReason   string `json:"review_reason" gorm:"type:varchar(255)"`
}

// MigrateCryptoPaymentOrders retains payment history while allowing an amount
// to be reused. Create the lookup index before removing the legacy unique index.
func MigrateCryptoPaymentOrders(db *gorm.DB) error {
	if err := db.AutoMigrate(&CryptoPaymentOrder{}); err != nil {
		return err
	}
	if db.Migrator().HasIndex(&CryptoPaymentOrder{}, "crypto_amount_reservation") {
		return db.Migrator().DropIndex(&CryptoPaymentOrder{}, "crypto_amount_reservation")
	}
	return nil
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
	order := &CryptoPaymentOrder{TradeNo: topup.TradeNo, UserID: topup.UserId, WalletID: walletID, CreditQuota: int64(creditQuota)}
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
		order.CreatedAt = time.Now().UnixMilli()
		order.ExpiresAt = order.CreatedAt + minutes*60000
		var uses []struct {
			AmountUnits  int64
			ExpiresAt    int64
			CompleteTime int64
		}
		err := tx.Table(tx.NamingStrategy.TableName("CryptoPaymentOrder")+" AS crypto_orders").
			Select("crypto_orders.amount_units, MAX(crypto_orders.expires_at) AS expires_at, COALESCE(MAX(crypto_topups.complete_time), 0) AS complete_time").
			Joins("LEFT JOIN (?) AS crypto_topups ON crypto_topups.trade_no = crypto_orders.trade_no", tx.Model(&TopUp{}).Select("trade_no, complete_time")).
			Where("crypto_orders.wallet_id = ? AND crypto_orders.amount_units > ? AND crypto_orders.amount_units < ?", walletID, baseUnits, baseUnits+cryptoPaymentAmountRange).
			Group("crypto_orders.amount_units").Scan(&uses).Error
		if err != nil {
			return err
		}
		reuseAfter := make(map[int64]int64, len(uses))
		for _, use := range uses {
			reuseAfter[use.AmountUnits] = max(use.ExpiresAt, use.CompleteTime*1000) + cryptoPaymentCooldown.Milliseconds()
		}
		var oldestAvailable int64
		for amount := baseUnits + cryptoPaymentAmountStep; amount < baseUnits+cryptoPaymentAmountRange; amount += cryptoPaymentAmountStep {
			availableAt, used := reuseAfter[amount]
			if !used {
				order.AmountUnits = amount
				break
			}
			if availableAt <= order.CreatedAt && (order.AmountUnits == 0 || availableAt < oldestAvailable) {
				order.AmountUnits, oldestAvailable = amount, availableAt
			}
		}
		if order.AmountUnits == 0 {
			return errors.New("No payment amount is currently available. Please try again later.")
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
	if transfer.Network == cryptopay.BSC {
		amount, err := cryptopay.BSCMicroAmount(transfer.RawAmount)
		if err != nil {
			return nil, false, err
		}
		if amount != transfer.Amount {
			return nil, false, fmt.Errorf("BSC transfer amount normalization mismatch")
		}
	} else if transfer.RawAmount != "" || transfer.Amount <= 0 {
		return nil, false, fmt.Errorf("invalid confirmed transfer amount")
	}
	if transfer.Timestamp <= 0 || transfer.TransactionID == "" || transfer.EventIndex < 0 {
		return nil, false, fmt.Errorf("invalid confirmed transfer")
	}
	record := &CryptoTransfer{Network: transfer.Network, Asset: transfer.Asset, TransactionID: transfer.TransactionID, EventIndex: transfer.EventIndex, Address: transfer.Address, AmountUnits: transfer.Amount, RawAmount: transfer.RawAmount, BlockTimestamp: transfer.Timestamp}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return nil, false, result.Error
	}
	inserted := result.RowsAffected == 1
	if err := DB.Where("network = ? AND transaction_id = ? AND event_index = ?", transfer.Network, transfer.TransactionID, transfer.EventIndex).First(record).Error; err != nil {
		return nil, false, err
	}
	if record.Asset != transfer.Asset || record.Address != transfer.Address || record.AmountUnits != transfer.Amount || record.RawAmount != transfer.RawAmount || record.BlockTimestamp != transfer.Timestamp {
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
		if err := lockForUpdate(tx).Where("network = ? AND asset = ? AND address = ?", transfer.Network, transfer.Asset, transfer.Address).First(&wallet).Error; err != nil {
			return err
		}
		var order CryptoPaymentOrder
		query := tx.Where("wallet_id = ?", wallet.ID)
		if manualTradeNo != "" {
			query = query.Where("trade_no = ?", manualTradeNo)
		} else {
			query = query.Where("amount_units = ? AND created_at <= ? AND expires_at >= ?", transfer.AmountUnits, transfer.BlockTimestamp, transfer.BlockTimestamp)
		}
		err := query.First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) && manualTradeNo == "" {
			review = "unmatched_amount"
			var count int64
			if err := tx.Model(&CryptoPaymentOrder{}).Where("wallet_id = ? AND amount_units = ?", wallet.ID, transfer.AmountUnits).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				review = "outside_payment_window"
			}
		} else if err != nil {
			return err
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
		RecordTopupLog(userID, common.NewMessage("USDT top-up succeeded, order: {{order}}, amount added: {{quota}}, receipt: {{receipt}}", map[string]any{"order": tradeNo, "quota": quota, "receipt": transferID}), callerIP, PaymentProviderCrypto, method)
	}
	return credited, nil
}

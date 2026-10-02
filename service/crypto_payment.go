package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptopay"
	"github.com/QuantumNous/new-api/setting"
)

// ScanCryptoPaymentsOnce resumes fixed pagination windows after restart. Recent
// windows overlap by a day, and a daily replay from wallet activation catches
// late indexing by the external API. Receipts are unique in the database.
func ScanCryptoPaymentsOnce(ctx context.Context, scanner cryptopay.Scanner) (int, error) {
	now := time.Now().UnixMilli()
	expired := model.DB.Model(&model.CryptoPaymentOrder{}).Select("trade_no").Where("expires_at < ?", now)
	if err := model.DB.Model(&model.TopUp{}).Where("payment_provider = ? AND status = ? AND trade_no IN (?)", model.PaymentProviderCrypto, common.TopUpStatusPending, expired).Update("status", common.TopUpStatusExpired).Error; err != nil {
		return 0, err
	}
	var wallets []model.CryptoWallet
	if err := model.DB.Order("id").Find(&wallets).Error; err != nil {
		return 0, err
	}
	processed := 0
	for _, wallet := range wallets {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		if wallet.Network != cryptopay.TRON || wallet.Asset != cryptopay.USDT {
			return processed, fmt.Errorf("unsupported crypto wallet %s/%s", wallet.Network, wallet.Asset)
		}
		if wallet.ScanTo == 0 {
			wallet.ScanFrom = max(wallet.CreatedAt-60000, wallet.ScannedThrough-24*60*60000)
			if wallet.LastFullScan < now-24*60*60000 {
				wallet.ScanFrom = wallet.CreatedAt - 60000
			}
			wallet.ScanTo = now - 120000
			if wallet.ScanTo < wallet.ScanFrom {
				continue
			}
			if err := model.DB.Model(&wallet).Updates(map[string]any{"scan_from": wallet.ScanFrom, "scan_to": wallet.ScanTo, "scan_cursor": ""}).Error; err != nil {
				return processed, err
			}
		}
		for range 10 {
			page, err := scanner.Scan(ctx, wallet.Address, wallet.ScanFrom, wallet.ScanTo, wallet.ScanCursor)
			if err != nil {
				return processed, fmt.Errorf("scan wallet %d: %w", wallet.ID, err)
			}
			for _, transfer := range page.Transfers {
				if transfer.Network != wallet.Network || transfer.Asset != wallet.Asset || transfer.Address != wallet.Address || transfer.Timestamp < wallet.ScanFrom || transfer.Timestamp > wallet.ScanTo {
					return processed, fmt.Errorf("scanner returned transfer outside wallet/window")
				}
				record, inserted, err := model.RecordCryptoTransfer(transfer)
				if err != nil {
					return processed, err
				}
				if _, err := model.SettleCryptoTransfer(record.ID, "", ""); err != nil {
					return processed, fmt.Errorf("settle crypto transfer %d: %w", record.ID, err)
				}
				if inserted {
					processed++
					if err := model.DB.First(record, record.ID).Error; err != nil {
						return processed, err
					}
					if record.ReviewReason != "" {
						logger.LogWarn(ctx, fmt.Sprintf("crypto payment requires review: transfer=%d tx=%s event=%d reason=%s", record.ID, record.TransactionID, record.EventIndex, record.ReviewReason))
					}
				}
			}
			if err := ctx.Err(); err != nil {
				return processed, err
			}
			if page.Next != "" {
				if page.Next == wallet.ScanCursor {
					return processed, fmt.Errorf("crypto scanner did not advance")
				}
				wallet.ScanCursor = page.Next
				if err := model.DB.Model(&wallet).Update("scan_cursor", page.Next).Error; err != nil {
					return processed, err
				}
				continue
			}
			updates := map[string]any{"scanned_through": wallet.ScanTo, "scan_to": 0, "scan_cursor": ""}
			if wallet.ScanFrom <= wallet.CreatedAt-60000 {
				updates["last_full_scan"] = now
			}
			if err := model.DB.Model(&wallet).Updates(updates).Error; err != nil {
				return processed, err
			}
			break
		}
	}
	return processed, nil
}

func RunCryptoPaymentScan(ctx context.Context) (int, error) {
	cfg, err := setting.GetCryptoPaymentConfig()
	if err != nil {
		return 0, err
	}
	if cfg.TRONAPIKey == "" {
		return 0, fmt.Errorf("TRON API key is required to monitor crypto payments")
	}
	return ScanCryptoPaymentsOnce(ctx, cryptopay.TRONScanner{APIKey: cfg.TRONAPIKey})
}

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptopay"
	"github.com/QuantumNous/new-api/setting"
)

// ScanCryptoPaymentsOnce resumes durable windows after restart. TRON history
// indexing is replayed with a day of overlap and daily full scans. BSC reads
// finalized block logs directly, so a minute of overlap suffices. A failed
// network does not stop other wallets, and database receipt keys deduplicate replay.
func ScanCryptoPaymentsOnce(ctx context.Context, scanners map[string]cryptopay.Scanner) (int, error) {
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
	var failures []error
	for _, wallet := range wallets {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		scanner, ok := scanners[wallet.Network]
		if !ok || wallet.Asset != cryptopay.USDT {
			failures = append(failures, fmt.Errorf("no scanner configured for crypto wallet %d (%s/%s)", wallet.ID, wallet.Network, wallet.Asset))
			continue
		}
		count, err := scanCryptoWallet(ctx, wallet, scanner, now)
		processed += count
		if err != nil {
			failures = append(failures, fmt.Errorf("scan wallet %d: %w", wallet.ID, err))
		}
	}
	return processed, errors.Join(failures...)
}

func scanCryptoWallet(ctx context.Context, wallet model.CryptoWallet, scanner cryptopay.Scanner, now int64) (int, error) {
	processed := 0
	if wallet.ScanTo == 0 {
		overlap := int64(60000)
		if wallet.Network == cryptopay.TRON {
			overlap = 24 * 60 * 60000
		}
		wallet.ScanFrom = max(wallet.CreatedAt-60000, wallet.ScannedThrough-overlap)
		if wallet.Network == cryptopay.TRON && wallet.LastFullScan < now-24*60*60000 {
			wallet.ScanFrom = wallet.CreatedAt - 60000
		}
		wallet.ScanTo = now
		if wallet.Network == cryptopay.TRON {
			wallet.ScanTo -= 120000
		}
		if wallet.ScanTo < wallet.ScanFrom {
			return processed, nil
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
		scannedThrough := wallet.ScanTo
		if wallet.Network == cryptopay.BSC {
			// A lagging finalized head must never advance the checkpoint past data
			// actually queried; the next window replays the boundary with overlap.
			if page.CompleteThrough < wallet.ScanFrom || page.CompleteThrough > wallet.ScanTo {
				return processed, fmt.Errorf("invalid BSC finalized scan checkpoint")
			}
			scannedThrough = page.CompleteThrough
		}
		updates := map[string]any{"scanned_through": scannedThrough, "scan_to": 0, "scan_cursor": ""}
		if wallet.ScanFrom <= wallet.CreatedAt-60000 {
			updates["last_full_scan"] = now
		}
		if err := model.DB.Model(&wallet).Updates(updates).Error; err != nil {
			return processed, err
		}
		break
	}
	return processed, nil
}

func RunCryptoPaymentScan(ctx context.Context) (int, error) {
	cfg, err := setting.GetCryptoPaymentConfig()
	if err != nil {
		return 0, err
	}
	scanners := map[string]cryptopay.Scanner{}
	if cfg.TRONAPIKey != "" {
		scanners[cryptopay.TRON] = cryptopay.TRONScanner{APIKey: cfg.TRONAPIKey}
	}
	if cfg.BSCRPCURL != "" {
		scanners[cryptopay.BSC] = cryptopay.BSCScanner{URL: cfg.BSCRPCURL}
	}
	return ScanCryptoPaymentsOnce(ctx, scanners)
}

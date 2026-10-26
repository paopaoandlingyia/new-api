package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptopay"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func cryptoPaymentTestDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var driver gorm.Dialector
	switch dialect {
	case "mysql":
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not configured")
		}
		driver = mysql.Open(dsn)
	case "postgres":
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not configured")
		}
		driver = postgres.Open(dsn)
	default:
		driver = sqlite.Open(filepath.Join(t.TempDir(), "crypto.db"))
	}
	db, err := gorm.Open(driver, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "crypto_test_"}})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Explicitly serialize SQLite writes; MySQL/PostgreSQL exercise row locks
	// through independent connections in the concurrent settlement cases.
	if dialect == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
	}
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis := common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldType, oldLogType)
		common.RedisEnabled = oldRedis
		for _, table := range []any{&model.CryptoTransfer{}, &model.CryptoPaymentOrder{}, &model.CryptoWallet{}, &model.TopUp{}, &model.User{}, &model.Log{}} {
			require.NoError(t, db.Migrator().DropTable(table))
		}
		require.NoError(t, sqlDB.Close())
	})
	var version string
	if dialect == "sqlite" {
		require.NoError(t, db.Raw("select sqlite_version()").Scan(&version).Error)
	} else {
		require.NoError(t, db.Raw("select version()").Scan(&version).Error)
	}
	t.Logf("%s version: %s", dialect, version)
	return db
}

func TestCryptoPaymentDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := cryptoPaymentTestDB(t, dialect)
			// User and TopUp schemas are unchanged from v1.0.0-rc.41. Seed the
			// existing payment tables before the additive crypto-table migration.
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}))
			user := &model.User{Username: "crypto-test", Quota: 100, Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(user).Error)
			legacy := &model.TopUp{UserId: user.Id, TradeNo: "legacy-paid", PaymentProvider: model.PaymentProviderEpay, Amount: 2, Money: 2, Status: common.TopUpStatusSuccess}
			require.NoError(t, db.Create(legacy).Error)
			for range 2 {
				require.NoError(t, db.AutoMigrate(&model.CryptoWallet{}, &model.CryptoPaymentOrder{}, &model.CryptoTransfer{}))
			}
			var preserved model.TopUp
			require.NoError(t, db.First(&preserved, legacy.Id).Error)
			assert.Equal(t, *legacy, preserved)
			wallet, err := model.EnsureCryptoWallet(cryptopay.TRON, cryptopay.USDT, cryptopay.TRONUSDTContract)
			require.NoError(t, err)
			again, err := model.EnsureCryptoWallet(wallet.Network, wallet.Asset, wallet.Address)
			require.NoError(t, err)
			assert.Equal(t, wallet.ID, again.ID)
			orders := make([]*model.CryptoPaymentOrder, 0, 6)
			for i := range 6 {
				topup := &model.TopUp{TradeNo: fmt.Sprintf("crypto-%d", i), UserId: user.Id, Amount: 10, PaymentProvider: model.PaymentProviderCrypto, PaymentMethod: model.PaymentProviderCrypto, Status: common.TopUpStatusPending}
				order, err := model.CreateCryptoPaymentOrder(wallet.ID, topup, 500, 10000000, 30)
				require.NoError(t, err)
				assert.EqualValues(t, 10000001+i, order.AmountUnits)
				orders = append(orders, order)
			}
			transfer := cryptopay.Transfer{Network: wallet.Network, Asset: wallet.Asset, Address: wallet.Address, TransactionID: strings.Repeat("a", 64), Amount: orders[0].AmountUnits, Timestamp: orders[0].CreatedAt + 1}
			record, inserted, err := model.RecordCryptoTransfer(transfer)
			require.NoError(t, err)
			assert.True(t, inserted)
			duplicate, inserted, err := model.RecordCryptoTransfer(transfer)
			require.NoError(t, err)
			assert.False(t, inserted)
			assert.Equal(t, record.ID, duplicate.ID)
			transfer.Amount++
			_, _, err = model.RecordCryptoTransfer(transfer)
			require.ErrorContains(t, err, "conflicting")
			var wg sync.WaitGroup
			errorsCh := make(chan error, 2)
			for range 2 {
				wg.Go(func() { _, err := model.SettleCryptoTransfer(record.ID, "", "test"); errorsCh <- err })
			}
			wg.Wait()
			close(errorsCh)
			for err := range errorsCh {
				require.NoError(t, err)
			}
			require.NoError(t, db.First(user, user.Id).Error)
			assert.Equal(t, 600, user.Quota, "repeated delivery must credit once")
			var paymentLog model.Log
			require.NoError(t, db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).First(&paymentLog).Error)
			var logDetails struct {
				ContentParts []*common.Message `json:"content_parts"`
			}
			require.NoError(t, common.UnmarshalJsonStr(paymentLog.Other, &logDetails))
			require.Len(t, logDetails.ContentParts, 1)
			assert.Equal(t, "USDT top-up succeeded, order: {{order}}, amount added: {{quota}}, receipt: {{receipt}}", logDetails.ContentParts[0].Key)
			assert.Equal(t, map[string]any{"order": orders[0].TradeNo, "quota": float64(500), "receipt": float64(record.ID)}, logDetails.ContentParts[0].Params)
			assert.Equal(t, fmt.Sprintf("USDT top-up succeeded, order: %s, amount added: 500, receipt: %d", orders[0].TradeNo, record.ID), paymentLog.Content)
			require.Error(t, model.ManualCompleteTopUp(orders[1].TradeNo, "test"), "generic manual completion must not bypass receipt consumption")
			_, err = model.SettleCryptoTransfer(record.ID, orders[1].TradeNo, "test")
			require.ErrorContains(t, err, "another order")
			// Two logs in one transaction are separate payments.
			transfer.Amount, transfer.EventIndex = orders[1].AmountUnits, 1
			transfer.Timestamp = orders[1].CreatedAt + 1
			second, _, err := model.RecordCryptoTransfer(transfer)
			require.NoError(t, err)
			credited, err := model.SettleCryptoTransfer(second.ID, "", "test")
			require.NoError(t, err)
			assert.True(t, credited)
			// A transfer included during the window still settles if scanning
			// resumes after expiry, since matching uses the block timestamp.
			require.NoError(t, db.Model(&model.TopUp{}).Where("trade_no = ?", orders[2].TradeNo).Update("status", common.TopUpStatusExpired).Error)
			transfer.TransactionID, transfer.EventIndex, transfer.Amount = strings.Repeat("b", 64), 0, orders[2].AmountUnits
			transfer.Timestamp = orders[2].ExpiresAt
			third, _, err := model.RecordCryptoTransfer(transfer)
			require.NoError(t, err)
			credited, err = model.SettleCryptoTransfer(third.ID, "", "test")
			require.NoError(t, err)
			assert.True(t, credited)
			// Late and incorrect amounts are retained for explicit reconciliation.
			transfer.TransactionID, transfer.Amount, transfer.Timestamp = strings.Repeat("c", 64), orders[3].AmountUnits, orders[3].ExpiresAt+1
			late, _, err := model.RecordCryptoTransfer(transfer)
			require.NoError(t, err)
			credited, err = model.SettleCryptoTransfer(late.ID, "", "test")
			require.NoError(t, err)
			assert.False(t, credited)
			require.NoError(t, db.First(late, late.ID).Error)
			assert.Equal(t, "outside_payment_window", late.ReviewReason)
			credited, err = model.SettleCryptoTransfer(late.ID, orders[3].TradeNo, "admin")
			require.NoError(t, err)
			assert.True(t, credited)
			transfer.TransactionID, transfer.Amount = strings.Repeat("d", 64), 9000000
			wrong, _, err := model.RecordCryptoTransfer(transfer)
			require.NoError(t, err)
			credited, err = model.SettleCryptoTransfer(wrong.ID, "", "test")
			require.NoError(t, err)
			assert.False(t, credited)
			require.NoError(t, db.First(wrong, wrong.ID).Error)
			assert.Equal(t, "unmatched_amount", wrong.ReviewReason)
			var logCount int64
			require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).Count(&logCount).Error)
			assert.EqualValues(t, 4, logCount, "every credited order must have an audit log")
			// If quota changed after checkout, the receipt/order remain unpaid.
			require.NoError(t, db.Model(user).Update("quota", common.MaxWalletQuota-1).Error)
			_, err = model.SettleCryptoTransfer(wrong.ID, orders[4].TradeNo, "admin")
			require.ErrorIs(t, err, model.ErrTopUpQuotaLimitExceeded)
			require.NoError(t, db.First(wrong, wrong.ID).Error)
			assert.Empty(t, wrong.TradeNo)
			var topup model.TopUp
			require.NoError(t, db.Where("trade_no = ?", orders[4].TradeNo).First(&topup).Error)
			assert.Equal(t, common.TopUpStatusPending, topup.Status)
			// Database uniqueness remains intact across repeated migrations.
			require.NoError(t, db.AutoMigrate(&model.CryptoWallet{}, &model.CryptoPaymentOrder{}, &model.CryptoTransfer{}))
			copyOrder := *orders[0]
			copyOrder.ID = 0
			copyOrder.TradeNo = "duplicate-amount"
			require.Error(t, db.Create(&copyOrder).Error)
			copyTransfer := *record
			copyTransfer.ID = 0
			require.Error(t, db.Create(&copyTransfer).Error)
			// A permanently reserved price range fails explicitly when exhausted.
			require.NoError(t, db.Create(&model.CryptoPaymentOrder{TradeNo: "last-slot", WalletID: wallet.ID, AmountUnits: 20009999}).Error)
			_, err = model.CreateCryptoPaymentOrder(wallet.ID, &model.TopUp{TradeNo: "exhausted", UserId: user.Id}, 500, 20000000, 30)
			require.ErrorContains(t, err, "exhausted")
		})
	}
}

type cryptoTestTransport func(*http.Request) (*http.Response, error)

func (transport cryptoTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestCryptoTRONReceiptVerification(t *testing.T) {
	txID := strings.Repeat("a", 64)
	contract := "a614f803b6fd780986a42c78ec9c7f77e6ded13c"
	topic := "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	to := strings.Repeat("0", 24) + contract
	amount := fmt.Sprintf("%064x", 10000001)
	validLog := fmt.Sprintf(`{"address":%q,"topics":[%q,%q,%q],"data":%q}`, contract, topic, strings.Repeat("0", 64), to, amount)
	for _, tc := range []struct {
		name, result, logs string
		want               int
		failure            bool
	}{
		{"two events in one transaction", "SUCCESS", validLog + "," + validLog, 2, false},
		{"failed execution", "REVERT", validLog, 0, true},
		{"fake USDT contract", "SUCCESS", strings.Replace(validLog, contract, strings.Repeat("b", 40), 1), 0, false},
		{"different recipient", "SUCCESS", strings.Replace(validLog, to, strings.Repeat("0", 64), 1), 0, false},
		{"malformed amount", "SUCCESS", strings.Replace(validLog, amount, "01", 1), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: cryptoTestTransport(func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "/walletsolidity/gettransactioninfobyid", req.URL.Path)
				assert.Equal(t, "query-key", req.Header.Get("TRON-PRO-API-KEY"))
				body := fmt.Sprintf(`{"id":%q,"blockTimeStamp":1000,"receipt":{"result":%q},"log":[%s]}`, txID, tc.result, tc.logs)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			transfers, err := (cryptopay.TRONScanner{APIKey: "query-key", Client: client}).Transaction(context.Background(), txID, cryptopay.TRONUSDTContract)
			if tc.failure {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, transfers, tc.want)
			for index, transfer := range transfers {
				assert.Equal(t, index, transfer.EventIndex)
				assert.EqualValues(t, 10000001, transfer.Amount)
			}
		})
	}
	_, err := cryptopay.TRONAddressBytes(cryptopay.TRONUSDTContract)
	require.NoError(t, err)
	_, err = cryptopay.TRONAddressBytes(cryptopay.TRONUSDTContract[:33] + "a")
	require.ErrorContains(t, err, "checksum")
}

type cryptoTestScanner struct {
	calls      []string
	failSecond bool
	transfers  []cryptopay.Transfer
}

func (scanner *cryptoTestScanner) Scan(_ context.Context, _ string, _, _ int64, cursor string) (cryptopay.Page, error) {
	scanner.calls = append(scanner.calls, cursor)
	if cursor == "" {
		return cryptopay.Page{Next: "second-page"}, nil
	}
	if scanner.failSecond {
		return cryptopay.Page{}, fmt.Errorf("query service unavailable")
	}
	return cryptopay.Page{Transfers: scanner.transfers}, nil
}

func TestCryptoScanResumesAfterFailure(t *testing.T) {
	db := cryptoPaymentTestDB(t, "sqlite")
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}, &model.CryptoWallet{}, &model.CryptoPaymentOrder{}, &model.CryptoTransfer{}))
	wallet, err := model.EnsureCryptoWallet(cryptopay.TRON, cryptopay.USDT, cryptopay.TRONUSDTContract)
	require.NoError(t, err)
	require.NoError(t, db.Model(wallet).Update("created_at", time.Now().Add(-time.Hour).UnixMilli()).Error)
	scanner := &cryptoTestScanner{failSecond: true}
	_, err = service.ScanCryptoPaymentsOnce(context.Background(), map[string]cryptopay.Scanner{cryptopay.TRON: scanner})
	require.ErrorContains(t, err, "unavailable")
	require.NoError(t, db.First(wallet, wallet.ID).Error)
	assert.Equal(t, "second-page", wallet.ScanCursor)
	assert.Zero(t, wallet.ScannedThrough)
	scanner.failSecond = false
	_, err = service.ScanCryptoPaymentsOnce(context.Background(), map[string]cryptopay.Scanner{cryptopay.TRON: scanner})
	require.NoError(t, err)
	assert.Equal(t, []string{"", "second-page", "second-page"}, scanner.calls)
	require.NoError(t, db.First(wallet, wallet.ID).Error)
	assert.Empty(t, wallet.ScanCursor)
	assert.Positive(t, wallet.ScannedThrough)
}

func TestCryptoCheckoutOwnershipAndValidation(t *testing.T) {
	db := cryptoPaymentTestDB(t, "sqlite")
	oldMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	common.IsMasterNode = oldMaster
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}, &model.CryptoWallet{}, &model.CryptoPaymentOrder{}, &model.CryptoTransfer{}))
	oldOptions, oldQuota := common.OptionMap, common.QuotaPerUnit
	oldDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	oldCompliance := *operation_setting.GetPaymentSetting()
	oldGroupRatios := common.TopupGroupRatio2JSONString()
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))
	common.QuotaPerUnit = 500000
	common.OptionMap = map[string]string{}
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.GetPaymentSetting().ComplianceConfirmed = true
	operation_setting.GetPaymentSetting().ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	operation_setting.GetPaymentSetting().AmountDiscount = nil
	t.Cleanup(func() {
		common.OptionMap = oldOptions
		common.QuotaPerUnit = oldQuota
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldDisplay
		*operation_setting.GetPaymentSetting() = oldCompliance
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(oldGroupRatios))
	})
	cfg := setting.CryptoPaymentConfig{Enabled: true, TRONAddress: cryptopay.TRONUSDTContract, TRONAPIKey: "query-only-key", BSCAddress: cryptopay.BSCUSDTContract, BSCRPCURL: "https://rpc.example.test/private-api-key", UnitPrice: "1", MinTopUp: 1, OrderMinutes: 30}
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	common.OptionMap[setting.CryptoPaymentConfigKey] = string(raw)
	user := model.User{Username: "checkout-user", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", user.Id) })
	router.POST("/pay", RequestCryptoPay)
	router.GET("/orders/:trade_no", GetCryptoOrder)
	router.GET("/config", GetCryptoPaymentConfig)
	router.PUT("/config", SaveCryptoPaymentConfig)
	router.GET("/options", GetOptions)
	router.PUT("/options", UpdateOption)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{"amount":10,"network":"tron"}`)))
	var checkout struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			TradeNo string `json:"trade_no"`
			Amount  string `json:"amount"`
			Network string `json:"network"`
			Address string `json:"address"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
	require.True(t, checkout.Success, response.Body.String())
	assert.Equal(t, "10.000001", checkout.Data.Amount)
	var order model.CryptoPaymentOrder
	require.NoError(t, db.Where("trade_no = ?", checkout.Data.TradeNo).First(&order).Error)
	assert.EqualValues(t, 5000000, order.CreditQuota)
	for _, path := range []string{"/config", "/options"} {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assert.NotContains(t, response.Body.String(), cfg.TRONAPIKey, "query credentials must never be returned")
		assert.NotContains(t, response.Body.String(), cfg.BSCRPCURL, "RPC credentials must never be returned")
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/options", strings.NewReader(`{"key":"CryptoPaymentConfig","value":"{}"}`)))
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
	assert.False(t, checkout.Success, "generic option writes must not bypass crypto configuration validation")
	user.Id++
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/orders/"+order.TradeNo, nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
	user.Id--
	for _, body := range []string{`{"amount":0,"network":"tron"}`, `{"amount":-1,"network":"tron"}`, `{"amount":9223372036854775807,"network":"tron"}`, `{"amount":10}`, `{"amount":10,"network":"bsc"}`} {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(body)))
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
		assert.False(t, checkout.Success, body)
	}
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	saveCfg := cfg
	saveCfg.BSCEnabled = true
	saveCfg.TRONAPIKey, saveCfg.BSCRPCURL = "", ""
	saveCfg.BSCAddress = "0x55D398326F99059FF775485246999027B3197955"
	raw, err = common.Marshal(saveCfg)
	require.NoError(t, err)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(string(raw))))
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
	require.True(t, checkout.Success, response.Body.String())
	stored, err := setting.GetCryptoPaymentConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.TRONAPIKey, stored.TRONAPIKey)
	assert.Equal(t, cfg.BSCRPCURL, stored.BSCRPCURL)
	assert.Equal(t, cryptopay.BSCUSDTContract, stored.BSCAddress)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{"amount":10,"network":"bsc"}`)))
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
	require.True(t, checkout.Success, response.Body.String())
	assert.Equal(t, "bsc", checkout.Data.Network)
	assert.Equal(t, cryptopay.BSCUSDTContract, checkout.Data.Address)
	assert.Equal(t, "10.000001", checkout.Data.Amount, "each network reserves its own amounts")
	bscTradeNo := checkout.Data.TradeNo
	stored.BSCEnabled = false
	raw, err = common.Marshal(stored)
	require.NoError(t, err)
	common.OptionMap[setting.CryptoPaymentConfigKey] = string(raw)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/orders/"+bscTradeNo, nil))
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
	assert.True(t, checkout.Success, "disabling new BSC orders preserves existing checkout details")
	assert.Equal(t, "bsc", checkout.Data.Network)
	cfg.Enabled = false
	raw, err = common.Marshal(cfg)
	require.NoError(t, err)
	common.OptionMap[setting.CryptoPaymentConfigKey] = string(raw)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/pay", strings.NewReader(`{"amount":10,"network":"tron"}`)))
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &checkout))
	assert.False(t, checkout.Success)
}

func TestCryptoBSCPaymentPrecision(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := cryptoPaymentTestDB(t, dialect)
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}, &model.CryptoWallet{}, &model.CryptoPaymentOrder{}, &model.CryptoTransfer{}))
			// Reproduce the previously deployed transfer schema, then migrate twice.
			require.NoError(t, db.Migrator().DropColumn(&model.CryptoTransfer{}, "RawAmount"))
			legacy := map[string]any{"network": "tron", "asset": "usdt", "transaction_id": "legacy-tron", "event_index": 0, "address": cryptopay.TRONUSDTContract, "amount_units": 10000001, "block_timestamp": int64(1700000000000)}
			require.NoError(t, db.Table("crypto_test_crypto_transfers").Create(legacy).Error)
			for range 2 {
				require.NoError(t, db.AutoMigrate(&model.CryptoTransfer{}))
			}
			var preserved model.CryptoTransfer
			require.NoError(t, db.Where("transaction_id = ?", "legacy-tron").First(&preserved).Error)
			assert.EqualValues(t, 10000001, preserved.AmountUnits)
			assert.Empty(t, preserved.RawAmount)
			user := model.User{Username: "bsc-user", Status: common.UserStatusEnabled}
			require.NoError(t, db.Create(&user).Error)
			wallet, err := model.EnsureCryptoWallet(cryptopay.BSC, cryptopay.USDT, cryptopay.BSCUSDTContract)
			require.NoError(t, err)
			tron, err := model.EnsureCryptoWallet(cryptopay.TRON, cryptopay.USDT, cryptopay.TRONUSDTContract)
			require.NoError(t, err)
			order, err := model.CreateCryptoPaymentOrder(wallet.ID, &model.TopUp{TradeNo: "bsc-order", UserId: user.Id, PaymentProvider: model.PaymentProviderCrypto, Status: common.TopUpStatusPending}, 500, 10000000, 30)
			require.NoError(t, err)
			tronOrder, err := model.CreateCryptoPaymentOrder(tron.ID, &model.TopUp{TradeNo: "tron-order", UserId: user.Id, PaymentProvider: model.PaymentProviderCrypto, Status: common.TopUpStatusPending}, 500, 10000000, 30)
			require.NoError(t, err)
			assert.Equal(t, order.AmountUnits, tronOrder.AmountUnits)
			dust := cryptopay.Transfer{Network: cryptopay.BSC, Asset: cryptopay.USDT, Address: wallet.Address, TransactionID: "0x" + strings.Repeat("a", 64), RawAmount: "10000001000000000001", Timestamp: order.CreatedAt + 1}
			receipt, inserted, err := model.RecordCryptoTransfer(dust)
			require.NoError(t, err)
			assert.True(t, inserted)
			credited, err := model.SettleCryptoTransfer(receipt.ID, "", "")
			require.NoError(t, err)
			assert.False(t, credited)
			require.NoError(t, db.First(receipt, receipt.ID).Error)
			assert.Equal(t, "unmatched_amount", receipt.ReviewReason)
			assert.Equal(t, dust.RawAmount, receipt.RawAmount)
			_, err = model.SettleCryptoTransfer(receipt.ID, tronOrder.TradeNo, "admin")
			require.Error(t, err, "manual reconciliation must not cross networks")
			exact := dust
			exact.EventIndex = 1
			exact.Amount = 10000001
			exact.RawAmount = "10000001000000000000"
			exactReceipt, _, err := model.RecordCryptoTransfer(exact)
			require.NoError(t, err)
			for range 2 {
				credited, err = model.SettleCryptoTransfer(exactReceipt.ID, "", "")
				require.NoError(t, err)
			}
			require.NoError(t, db.First(&user, user.Id).Error)
			assert.Equal(t, 500, user.Quota)
			var unpaid model.TopUp
			require.NoError(t, db.Where("trade_no = ?", tronOrder.TradeNo).First(&unpaid).Error)
			assert.Equal(t, common.TopUpStatusPending, unpaid.Status)
			replay, inserted, err := model.RecordCryptoTransfer(exact)
			require.NoError(t, err)
			assert.False(t, inserted)
			assert.Equal(t, exactReceipt.ID, replay.ID)
			exact.RawAmount = "10000001000000000001"
			exact.Amount = 0
			_, _, err = model.RecordCryptoTransfer(exact)
			require.ErrorContains(t, err, "conflicting")
		})
	}
}

func TestCryptoBSCFinalizedScanner(t *testing.T) {
	const base = int64(1700000000000)
	address := cryptopay.BSCUSDTContract
	hash := "0x" + strings.Repeat("a", 64)
	chain := "0x38"
	badHash := false
	invalidLog := ""
	client := &http.Client{Transport: cryptoTestTransport(func(request *http.Request) (*http.Response, error) {
		var call struct {
			ID     int                 `json:"id"`
			Method string              `json:"method"`
			Params []common.RawMessage `json:"params"`
		}
		require.NoError(t, common.DecodeJson(request.Body, &call))
		var result any
		switch call.Method {
		case "eth_chainId":
			result = chain
		case "eth_getBlockByNumber":
			var tag string
			require.NoError(t, common.Unmarshal(call.Params[0], &tag))
			number := int64(1003)
			if tag != "finalized" {
				var err error
				number, err = strconv.ParseInt(strings.TrimPrefix(tag, "0x"), 16, 64)
				require.NoError(t, err)
			}
			result = map[string]any{"number": fmt.Sprintf("0x%x", number), "timestamp": fmt.Sprintf("0x%x", base/1000+number/2), "hash": hash}
		case "eth_getLogs":
			var filter struct {
				From    string `json:"fromBlock"`
				To      string `json:"toBlock"`
				Address string `json:"address"`
				Topics  []any  `json:"topics"`
			}
			require.NoError(t, common.Unmarshal(call.Params[0], &filter))
			assert.Equal(t, cryptopay.BSCUSDTContract, filter.Address)
			logs := []any{}
			if filter.From == "0x3e8" {
				for index, raw := range []string{"10000001000000000001", "10000001000000000000"} {
					value, ok := new(big.Int).SetString(raw, 10)
					require.True(t, ok)
					blockHash := hash
					if badHash {
						blockHash = "0x" + strings.Repeat("b", 64)
					}
					logs = append(logs, map[string]any{"address": address, "topics": []string{"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef", "0x" + strings.Repeat("0", 64), "0x" + strings.Repeat("0", 24) + address[2:]}, "data": fmt.Sprintf("0x%064x", value), "transactionHash": hash, "blockNumber": fmt.Sprintf("0x%x", 1000+index), "blockHash": blockHash, "logIndex": fmt.Sprintf("0x%x", index)})
				}
			}
			if len(logs) > 0 {
				item := logs[0].(map[string]any)
				switch invalidLog {
				case "contract":
					item["address"] = "0x" + strings.Repeat("b", 40)
				case "recipient":
					item["topics"].([]string)[2] = "0x" + strings.Repeat("0", 64)
				case "removed":
					item["removed"] = true
				case "amount":
					item["data"] = "0x1"
				case "block":
					item["blockNumber"] = "0x3e7"
				}
			}
			result = logs
		default:
			t.Fatalf("unexpected RPC method %s", call.Method)
		}
		body, err := common.Marshal(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result})
		require.NoError(t, err)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}
	scanner := cryptopay.BSCScanner{URL: "https://rpc.example.test/secret", Client: client}
	page, err := scanner.Scan(context.Background(), address, base, base+500000, "")
	require.NoError(t, err)
	assert.Empty(t, page.Transfers)
	assert.Equal(t, "1000", page.Next)
	page, err = scanner.Scan(context.Background(), address, base, base+500000, page.Next)
	require.NoError(t, err)
	require.Len(t, page.Transfers, 2)
	assert.Empty(t, page.Next)
	assert.EqualValues(t, 0, page.Transfers[0].Amount)
	assert.Equal(t, "10000001000000000001", page.Transfers[0].RawAmount)
	assert.EqualValues(t, 10000001, page.Transfers[1].Amount)
	assert.EqualValues(t, base+500000, page.Transfers[1].Timestamp, "blocks sharing the window's final timestamp must both be included")
	for _, name := range []string{"contract", "recipient", "removed", "amount", "block"} {
		t.Run("reject_"+name, func(t *testing.T) {
			invalidLog = name
			_, err := scanner.Scan(context.Background(), address, base, base+500000, "1000")
			require.Error(t, err)
		})
	}
	invalidLog = ""
	_, err = scanner.Scan(context.Background(), address, base, base+500000, "not-a-block")
	require.ErrorContains(t, err, "cursor")
	badHash = true
	_, err = scanner.Scan(context.Background(), address, base, base+500000, "1000")
	require.ErrorContains(t, err, "hash mismatch")
	badHash = false
	chain = "0x1"
	_, err = scanner.Scan(context.Background(), address, base, base+500000, "")
	require.ErrorContains(t, err, "chain ID 56")
	chain = "0x38"
	_, err = scanner.Scan(context.Background(), address, base, base+502000, "")
	require.ErrorContains(t, err, "finalized head")
}

func TestCryptoBSCScanRecoveryAndNetworkIsolation(t *testing.T) {
	db := cryptoPaymentTestDB(t, "sqlite")
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.Log{}, &model.CryptoWallet{}, &model.CryptoPaymentOrder{}, &model.CryptoTransfer{}))
	user := model.User{Username: "scan-user", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	tron, err := model.EnsureCryptoWallet(cryptopay.TRON, cryptopay.USDT, cryptopay.TRONUSDTContract)
	require.NoError(t, err)
	bsc, err := model.EnsureCryptoWallet(cryptopay.BSC, cryptopay.USDT, cryptopay.BSCUSDTContract)
	require.NoError(t, err)
	for _, wallet := range []*model.CryptoWallet{tron, bsc} {
		require.NoError(t, db.Model(wallet).Update("created_at", time.Now().Add(-time.Hour).UnixMilli()).Error)
	}
	order, err := model.CreateCryptoPaymentOrder(bsc.ID, &model.TopUp{TradeNo: "scan-bsc", UserId: user.Id, PaymentProvider: model.PaymentProviderCrypto, Status: common.TopUpStatusPending}, 500, 10000000, 30)
	require.NoError(t, err)
	require.NoError(t, db.Model(order).Update("created_at", time.Now().Add(-5*time.Minute).UnixMilli()).Error)
	transfer := cryptopay.Transfer{Network: cryptopay.BSC, Asset: cryptopay.USDT, Address: bsc.Address, TransactionID: "0x" + strings.Repeat("d", 64), Timestamp: time.Now().Add(-4 * time.Minute).UnixMilli(), RawAmount: "10000001000000000001"}
	exact := transfer
	exact.EventIndex = 1
	exact.Amount = 10000001
	exact.RawAmount = "10000001000000000000"
	tronScanner := &cryptoTestScanner{failSecond: true}
	bscScanner := &cryptoTestScanner{failSecond: true, transfers: []cryptopay.Transfer{transfer, exact}}
	scanners := map[string]cryptopay.Scanner{cryptopay.TRON: tronScanner, cryptopay.BSC: bscScanner}
	_, err = service.ScanCryptoPaymentsOnce(context.Background(), scanners)
	require.ErrorContains(t, err, "unavailable")
	require.NoError(t, db.First(bsc, bsc.ID).Error)
	assert.Equal(t, "second-page", bsc.ScanCursor)
	bscScanner.failSecond = false
	count, err := service.ScanCryptoPaymentsOnce(context.Background(), scanners)
	require.ErrorContains(t, err, "unavailable", "TRON still fails independently")
	assert.Equal(t, 2, count, "an unmatched precision must not prevent the next transfer from settling")
	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, 500, user.Quota)
	require.NoError(t, db.First(bsc, bsc.ID).Error)
	assert.Empty(t, bsc.ScanCursor)
	assert.Positive(t, bsc.ScannedThrough)
	assert.Equal(t, []string{"", "second-page", "second-page"}, bscScanner.calls)
}

func TestCryptoBSCAmountValidation(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		micro   int64
		invalid bool
	}{
		{"10000001000000000000", 10000001, false},
		{"10000001000000000001", 0, false},
		{"1", 0, false},
		{"115792089237316195423570985008687907853269984665640564039457584007913129639935", 0, false},
		{"0", 0, true}, {"-1", 0, true}, {"01", 0, true}, {"1e18", 0, true},
		{"115792089237316195423570985008687907853269984665640564039457584007913129639936", 0, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			value, err := cryptopay.BSCMicroAmount(tc.raw)
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.micro, value)
			}
		})
	}
}

package controller

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cryptopay"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"
)

func isCryptoTopUpEnabled(cfg setting.CryptoPaymentConfig) bool {
	return cfg.Enabled && cfg.TRONAddress != "" && cfg.TRONAPIKey != "" && operation_setting.IsPaymentComplianceConfirmed()
}

func GetCryptoPaymentConfig(c *gin.Context) {
	cfg, err := setting.GetCryptoPaymentConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keySet := cfg.TRONAPIKey != ""
	cfg.TRONAPIKey = ""
	common.ApiSuccess(c, gin.H{"config": cfg, "api_key_set": keySet})
}

func SaveCryptoPaymentConfig(c *gin.Context) {
	var cfg setting.CryptoPaymentConfig
	if err := common.DecodeJson(c.Request.Body, &cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg.TRONAddress = strings.TrimSpace(cfg.TRONAddress)
	cfg.TRONAPIKey = strings.TrimSpace(cfg.TRONAPIKey)
	if cfg.TRONAPIKey == "" {
		previous, err := setting.GetCryptoPaymentConfig()
		if err != nil {
			common.ApiError(c, err)
			return
		}
		// GET deliberately omits the query API credential; an unchanged blank
		// password field preserves it, as with existing payment settings.
		cfg.TRONAPIKey = previous.TRONAPIKey
	}
	if err := cfg.Validate(); err != nil {
		common.ApiError(c, err)
		return
	}
	if cfg.TRONAddress != "" {
		if _, err := cryptopay.TRONAddressBytes(cfg.TRONAddress); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if cfg.Enabled && (cfg.TRONAddress == "" || cfg.TRONAPIKey == "") {
		common.ApiErrorMsg(c, "TRON address and query API key are required")
		return
	}
	if cfg.Enabled && !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorMsg(c, "Complete the existing payment compliance confirmation first")
		return
	}
	if cfg.TRONAddress != "" {
		if _, err := model.EnsureCryptoWallet(cryptopay.TRON, cryptopay.USDT, cfg.TRONAddress); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	raw, err := common.Marshal(cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOptionsBulk(map[string]string{setting.CryptoPaymentConfigKey: string(raw)}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// The nominal price excludes the identifying suffix (< 0.01 USDT). Quota is
// snapshotted before checkout; changing settings cannot change an existing order.
func cryptoTopUpPrice(amount int64, userID int, cfg setting.CryptoPaymentConfig) (int, int64, error) {
	if err := cfg.Validate(); err != nil {
		return 0, 0, err
	}
	if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return 0, 0, fmt.Errorf("invalid quota conversion rate")
	}
	quota, err := validateTopUpQuota(amount)
	if err != nil {
		return 0, 0, err
	}
	if err := model.ValidateTopUpQuotaCapacity(userID, quota); err != nil {
		return 0, 0, err
	}
	dollars := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dollars = dollars.Div(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	if dollars.LessThan(decimal.NewFromInt(cfg.MinTopUp)) {
		return 0, 0, fmt.Errorf("minimum top-up is %d", cfg.MinTopUp)
	}
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		return 0, 0, err
	}
	ratio := common.GetTopupGroupRatio(group)
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0, 0, fmt.Errorf("invalid top-up group ratio")
	}
	price, err := decimal.NewFromString(cfg.UnitPrice)
	if err != nil {
		return 0, 0, err
	}
	price = dollars.Mul(price).Mul(decimal.NewFromFloat(ratio))
	if discount, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if discount <= 0 || discount > 1 || math.IsNaN(discount) {
			return 0, 0, fmt.Errorf("invalid top-up discount")
		}
		price = price.Mul(decimal.NewFromFloat(discount))
	}
	price = price.Round(2)
	if !price.IsPositive() || price.GreaterThan(decimal.NewFromInt(1000000)) {
		return 0, 0, fmt.Errorf("payment must be greater than zero and at most 1000000 USDT")
	}
	return quota, price.Shift(6).IntPart(), nil
}

func RequestCryptoAmount(c *gin.Context) {
	var req AmountRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, err := setting.GetCryptoPaymentConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	_, units, err := cryptoTopUpPrice(req.Amount, c.GetInt("id"), cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, decimal.NewFromInt(units).Shift(-6).StringFixed(2))
}

func RequestCryptoPay(c *gin.Context) {
	cfg, err := setting.GetCryptoPaymentConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !isCryptoTopUpEnabled(cfg) {
		common.ApiErrorMsg(c, "USDT-TRC20 payments are disabled")
		return
	}
	var req AmountRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	quota, base, err := cryptoTopUpPrice(req.Amount, c.GetInt("id"), cfg)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	wallet, err := model.EnsureCryptoWallet(cryptopay.TRON, cryptopay.USDT, cfg.TRONAddress)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	topup := &model.TopUp{UserId: c.GetInt("id"), TradeNo: "crypto-" + randstr.Hex(32), Amount: req.Amount, PaymentMethod: model.PaymentProviderCrypto, PaymentProvider: model.PaymentProviderCrypto, CreateTime: common.GetTimestamp(), Status: common.TopUpStatusPending}
	order, err := model.CreateCryptoPaymentOrder(wallet.ID, topup, quota, base, cfg.OrderMinutes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cryptoOrderResponse(order, wallet, topup.Status))
}

func cryptoOrderResponse(order *model.CryptoPaymentOrder, wallet *model.CryptoWallet, status string) gin.H {
	if status == common.TopUpStatusPending && order.ExpiresAt < time.Now().UnixMilli() {
		status = common.TopUpStatusExpired
	}
	return gin.H{"trade_no": order.TradeNo, "network": wallet.Network, "asset": wallet.Asset, "address": wallet.Address, "amount": decimal.NewFromInt(order.AmountUnits).Shift(-6).StringFixed(6), "credit_quota": order.CreditQuota, "expires_at": order.ExpiresAt, "status": status}
}

func GetCryptoOrder(c *gin.Context) {
	var order model.CryptoPaymentOrder
	err := model.DB.Where("trade_no = ? AND user_id = ?", c.Param("trade_no"), c.GetInt("id")).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Order not found"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var wallet model.CryptoWallet
	var topup model.TopUp
	if err := model.DB.First(&wallet, order.WalletID).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cryptoOrderResponse(&order, &wallet, topup.Status))
}

func ListCryptoTransfers(c *gin.Context) {
	page := common.GetPageQuery(c)
	var records []model.CryptoTransfer
	var total int64
	query := model.DB.Model(&model.CryptoTransfer{}).Where("trade_no = ?", "")
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := query.Order("id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&records).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	type transferResponse struct {
		model.CryptoTransfer
		Amount string `json:"amount"`
	}
	items := make([]transferResponse, 0, len(records))
	for _, record := range records {
		items = append(items, transferResponse{CryptoTransfer: record, Amount: decimal.NewFromInt(record.AmountUnits).Shift(-6).StringFixed(6)})
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func ReconcileCryptoTransfer(c *gin.Context) {
	var req struct {
		TransferID int64  `json:"transfer_id"`
		TradeNo    string `json:"trade_no"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.TransferID <= 0 || strings.TrimSpace(req.TradeNo) == "" {
		common.ApiErrorMsg(c, "Confirmed transfer and order number are required")
		return
	}
	if _, err := model.SettleCryptoTransfer(req.TransferID, req.TradeNo, c.ClientIP()); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

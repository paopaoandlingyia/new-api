package setting

import (
	"fmt"
	"net/url"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cryptopay"
	"github.com/shopspring/decimal"
)

const CryptoPaymentConfigKey = "CryptoPaymentConfig"

// CryptoPaymentConfig contains no wallet private key. UnitPrice is USDT per
// dollar of gateway credit, independent of the interface's display currency.
type CryptoPaymentConfig struct {
	Enabled      bool   `json:"enabled"`
	TRONAddress  string `json:"tron_address"`
	TRONAPIKey   string `json:"tron_api_key"`
	BSCEnabled   bool   `json:"bsc_enabled"`
	BSCAddress   string `json:"bsc_address"`
	BSCRPCURL    string `json:"bsc_rpc_url"`
	UnitPrice    string `json:"unit_price"`
	MinTopUp     int64  `json:"min_topup"`
	OrderMinutes int64  `json:"order_minutes"`
}

func GetCryptoPaymentConfig() (CryptoPaymentConfig, error) {
	cfg := CryptoPaymentConfig{UnitPrice: "1", MinTopUp: 1, OrderMinutes: 30}
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[CryptoPaymentConfigKey]
	common.OptionMapRWMutex.RUnlock()
	if raw != "" {
		if err := common.UnmarshalJsonStr(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("decode crypto payment configuration: %w", err)
		}
	}
	return cfg, nil
}

func (cfg CryptoPaymentConfig) Networks() []string {
	networks := []string{}
	if cfg.TRONAddress != "" && cfg.TRONAPIKey != "" {
		networks = append(networks, cryptopay.TRON)
	}
	if cfg.BSCEnabled && cfg.BSCAddress != "" && cfg.BSCRPCURL != "" {
		networks = append(networks, cryptopay.BSC)
	}
	return networks
}

func (cfg CryptoPaymentConfig) Validate() error {
	if cfg.BSCAddress != "" {
		if _, err := cryptopay.BSCAddress(cfg.BSCAddress); err != nil {
			return err
		}
	}
	if cfg.BSCRPCURL != "" {
		endpoint, err := url.Parse(cfg.BSCRPCURL)
		if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.Fragment != "" || endpoint.User != nil {
			return fmt.Errorf("BSC RPC URL must use HTTPS without user credentials or a fragment")
		}
	}
	if cfg.BSCEnabled && (cfg.BSCAddress == "" || cfg.BSCRPCURL == "") {
		return fmt.Errorf("BSC address and RPC URL are required")
	}

	price, err := decimal.NewFromString(cfg.UnitPrice)
	if err != nil || !price.IsPositive() || price.GreaterThan(decimal.NewFromInt(10000)) {
		return fmt.Errorf("USDT unit price must be greater than 0 and at most 10000")
	}
	if cfg.MinTopUp < 1 || cfg.MinTopUp > 1000000 {
		return fmt.Errorf("minimum top-up must be between 1 and 1000000")
	}
	if cfg.OrderMinutes < 5 || cfg.OrderMinutes > 1440 {
		return fmt.Errorf("payment window must be between 5 and 1440 minutes")
	}
	return nil
}

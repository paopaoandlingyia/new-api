// Package cryptopay reads confirmed transfers without access to wallet keys.
// Network adapters expose transfer facts; order allocation and quota accounting
// belong to the host's model and service packages.
package cryptopay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	TRON             = "tron"
	USDT             = "usdt"
	TRONUSDTContract = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	transferTopic    = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	base58Alphabet   = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

type Transfer struct {
	Network       string
	Asset         string
	TransactionID string
	EventIndex    int
	Address       string
	Amount        int64  // Exact micro-USDT, or zero when RawAmount cannot identify an order.
	RawAmount     string // BSC uint256 in token base units (18 decimals); empty for legacy TRON records.
	Timestamp     int64  // Block timestamp in milliseconds.
}

type Page struct {
	Transfers []Transfer
	Next      string
	// CompleteThrough is the finalized timestamp covered when a BSC page has no Next.
	CompleteThrough int64
}

type Scanner interface {
	Scan(context.Context, string, int64, int64, string) (Page, error)
}

type TRONScanner struct {
	APIKey string
	Client *http.Client
}

func (s TRONScanner) request(ctx context.Context, method, path string, body []byte, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, "https://api.trongrid.io"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("TRON-PRO-API-KEY", s.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("TRON API request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return fmt.Errorf("read TRON response: %w", err)
	}
	if len(data) > 8<<20 {
		return fmt.Errorf("TRON response exceeds 8 MiB")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TRON API HTTP %d", resp.StatusCode)
	}
	if err := common.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode TRON response: %w", err)
	}
	return nil
}

func (s TRONScanner) Scan(ctx context.Context, address string, start, end int64, cursor string) (Page, error) {
	if _, err := TRONAddressBytes(address); err != nil {
		return Page{}, err
	}
	params := url.Values{
		"only_confirmed": {"true"}, "only_to": {"true"}, "limit": {"200"},
		"contract_address": {TRONUSDTContract}, "order_by": {"block_timestamp,asc"},
		"min_timestamp": {strconv.FormatInt(start, 10)}, "max_timestamp": {strconv.FormatInt(end, 10)},
	}
	if cursor != "" {
		params.Set("fingerprint", cursor)
	}
	var history struct {
		Success bool `json:"success"`
		Data    []struct {
			TransactionID string `json:"transaction_id"`
		} `json:"data"`
		Meta struct {
			Fingerprint string `json:"fingerprint"`
			Links       struct {
				Next string `json:"next"`
			} `json:"links"`
		} `json:"meta"`
	}
	if err := s.request(ctx, http.MethodGet, "/v1/accounts/"+address+"/transactions/trc20?"+params.Encode(), nil, &history); err != nil {
		return Page{}, err
	}
	if !history.Success {
		return Page{}, fmt.Errorf("TRON transfer history was unsuccessful")
	}
	page := Page{}
	if history.Meta.Links.Next != "" {
		page.Next = history.Meta.Fingerprint
		if page.Next == "" || page.Next == cursor {
			return Page{}, fmt.Errorf("TRON history returned an invalid pagination cursor")
		}
	}
	seen := make(map[string]bool)
	for _, item := range history.Data {
		if seen[item.TransactionID] {
			continue
		}
		seen[item.TransactionID] = true
		transfers, err := s.Transaction(ctx, item.TransactionID, address)
		if err != nil {
			return Page{}, err
		}
		for _, transfer := range transfers {
			if transfer.Timestamp >= start && transfer.Timestamp <= end {
				page.Transfers = append(page.Transfers, transfer)
			}
		}
	}
	return page, nil
}

// Transaction verifies the solidified execution receipt, rather than trusting
// a token name or an unconfirmed explorer entry. Each receipt log is distinct.
func (s TRONScanner) Transaction(ctx context.Context, txID, address string) ([]Transfer, error) {
	idBytes, err := hex.DecodeString(txID)
	if err != nil || len(idBytes) != 32 {
		return nil, fmt.Errorf("invalid TRON transaction ID")
	}
	to, err := TRONAddressBytes(address)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(map[string]string{"value": txID})
	if err != nil {
		return nil, err
	}
	var receipt struct {
		ID        string `json:"id"`
		Error     string `json:"Error"`
		Result    string `json:"result"`
		Timestamp int64  `json:"blockTimeStamp"`
		Receipt   struct {
			Result string `json:"result"`
		} `json:"receipt"`
		Log []struct {
			Address string   `json:"address"`
			Topics  []string `json:"topics"`
			Data    string   `json:"data"`
		} `json:"log"`
	}
	if err := s.request(ctx, http.MethodPost, "/walletsolidity/gettransactioninfobyid", body, &receipt); err != nil {
		return nil, err
	}
	if receipt.Error != "" {
		return nil, fmt.Errorf("TRON receipt error: %s", receipt.Error)
	}
	if receipt.ID != txID || receipt.Timestamp <= 0 {
		return nil, fmt.Errorf("solidified TRON receipt not available for %s", txID)
	}
	if receipt.Result == "FAILED" || receipt.Receipt.Result != "SUCCESS" {
		return nil, fmt.Errorf("TRON transaction %s did not execute successfully", txID)
	}
	contract, err := TRONAddressBytes(TRONUSDTContract)
	if err != nil {
		return nil, err
	}
	var transfers []Transfer
	for index, log := range receipt.Log {
		if log.Address != hex.EncodeToString(contract[1:]) || len(log.Topics) != 3 || log.Topics[0] != transferTopic {
			continue
		}
		recipient, err := hex.DecodeString(log.Topics[2])
		if err != nil || len(recipient) != 32 {
			return nil, fmt.Errorf("invalid USDT Transfer recipient in %s", txID)
		}
		if !bytes.Equal(recipient[:12], make([]byte, 12)) || !bytes.Equal(recipient[12:], to[1:]) {
			continue
		}
		value, err := hex.DecodeString(log.Data)
		if err != nil || len(value) != 32 {
			return nil, fmt.Errorf("invalid USDT Transfer amount in %s", txID)
		}
		amount := new(big.Int).SetBytes(value)
		if !amount.IsInt64() {
			return nil, fmt.Errorf("USDT Transfer amount exceeds int64 in %s", txID)
		}
		if amount.Sign() <= 0 {
			continue
		}
		transfers = append(transfers, Transfer{Network: TRON, Asset: USDT, TransactionID: txID, EventIndex: index, Address: address, Amount: amount.Int64(), Timestamp: receipt.Timestamp})
	}
	return transfers, nil
}

// TRONAddressBytes validates Base58Check and returns the 21-byte TRON address.
func TRONAddressBytes(address string) ([]byte, error) {
	if len(address) != 34 || address[0] != 'T' {
		return nil, fmt.Errorf("invalid TRON address")
	}
	n := new(big.Int)
	for _, char := range address {
		digit := strings.IndexRune(base58Alphabet, char)
		if digit < 0 {
			return nil, fmt.Errorf("invalid TRON address character")
		}
		n.Mul(n, big.NewInt(58)).Add(n, big.NewInt(int64(digit)))
	}
	raw := n.Bytes()
	if len(raw) != 25 || raw[0] != 0x41 {
		return nil, fmt.Errorf("invalid TRON address payload")
	}
	hash := sha256.Sum256(raw[:21])
	hash = sha256.Sum256(hash[:])
	if !bytes.Equal(hash[:4], raw[21:]) {
		return nil, fmt.Errorf("invalid TRON address checksum")
	}
	return raw[:21], nil
}

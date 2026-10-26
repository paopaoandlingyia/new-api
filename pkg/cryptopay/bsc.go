package cryptopay

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
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
	BSC             = "bsc"
	BSCUSDTContract = "0x55d398326f99059ff775485246999027b3197955"
)

// BSCAddress returns the canonical address used for database matching.
func BSCAddress(address string) (string, error) {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return "", fmt.Errorf("invalid BSC address")
	}
	decoded, err := hex.DecodeString(address[2:])
	if err != nil || bytes.Equal(decoded, make([]byte, 20)) {
		return "", fmt.Errorf("invalid BSC address")
	}
	return strings.ToLower(address), nil
}

// BSCMicroAmount preserves the original uint256 separately. Nonzero digits
// below micro-USDT, or amounts above int64, cannot identify a six-decimal order.
func BSCMicroAmount(raw string) (int64, error) {
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok || value.Sign() <= 0 || value.BitLen() > 256 || value.String() != raw {
		return 0, fmt.Errorf("invalid BSC token amount")
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value, big.NewInt(1000000000000), remainder)
	if remainder.Sign() != 0 || !quotient.IsInt64() {
		return 0, nil
	}
	return quotient.Int64(), nil
}

type BSCScanner struct {
	URL    string
	Client *http.Client
}

func (s BSCScanner) rpc(ctx context.Context, method string, params any, target any) error {
	body, err := common.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("invalid BSC RPC request")
	}
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		// RPC URLs may contain a provider credential. Retain the transport cause,
		// but never expose the credential-bearing URL in errors or task logs.
		var transport *url.Error
		if errors.As(err, &transport) {
			err = transport.Err
		}
		return fmt.Errorf("BSC RPC %s: %w", method, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil {
		return fmt.Errorf("read BSC RPC response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("BSC RPC HTTP %d", response.StatusCode)
	}
	if len(data) > 8<<20 {
		return fmt.Errorf("BSC RPC response exceeds 8 MiB")
	}
	var envelope struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      int               `json:"id"`
		Result  common.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := common.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode BSC RPC response: %w", err)
	}
	if envelope.Error != nil {
		// Providers may echo their credential-bearing URL. Keep diagnostic text
		// while redacting the configured endpoint and its query credentials.
		message := strings.ReplaceAll(envelope.Error.Message, s.URL, "[redacted RPC URL]")
		if endpoint, err := url.Parse(s.URL); err == nil {
			for _, values := range endpoint.Query() {
				for _, value := range values {
					if value != "" {
						message = strings.ReplaceAll(message, value, "[redacted]")
					}
				}
			}
		}
		return fmt.Errorf("BSC RPC %s error %d: %s", method, envelope.Error.Code, message)
	}
	if envelope.JSONRPC != "2.0" || envelope.ID != 1 || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return fmt.Errorf("missing or invalid BSC RPC %s result", method)
	}
	return common.Unmarshal(envelope.Result, target)
}

type bscBlock struct {
	Number    string `json:"number"`
	Timestamp string `json:"timestamp"`
	Hash      string `json:"hash"`
}

func bscNumber(value string) (int64, error) {
	if !strings.HasPrefix(value, "0x") || len(value) < 3 {
		return 0, fmt.Errorf("invalid BSC quantity")
	}
	n, err := strconv.ParseInt(value[2:], 16, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid BSC quantity %q", value)
	}
	return n, nil
}

func (s BSCScanner) block(ctx context.Context, tag string) (bscBlock, int64, int64, error) {
	var block bscBlock
	if err := s.rpc(ctx, "eth_getBlockByNumber", []any{tag, false}, &block); err != nil {
		return block, 0, 0, err
	}
	number, err := bscNumber(block.Number)
	if err != nil {
		return block, 0, 0, err
	}
	if strings.HasPrefix(tag, "0x") && tag != block.Number {
		return block, 0, 0, fmt.Errorf("BSC RPC returned the wrong block number")
	}
	timestamp, err := bscNumber(block.Timestamp)
	if err != nil {
		return block, 0, 0, err
	}
	if timestamp <= 0 || timestamp > int64(^uint64(0)>>1)/1000 || !bscHash(block.Hash) {
		return block, 0, 0, fmt.Errorf("invalid BSC block")
	}
	return block, number, timestamp * 1000, nil
}

func bscHash(hash string) bool {
	if len(hash) != 66 || !strings.HasPrefix(hash, "0x") {
		return false
	}
	_, err := hex.DecodeString(hash[2:])
	return err == nil
}

// Scan searches finalized block history for a durable timestamp window. A page
// covers at most 1000 blocks and its cursor is the next block, not a provider's
// ephemeral pagination token. An unavailable finalized head fails without moving
// the checkpoint; no unconfirmed payment can enter settlement.
func (s BSCScanner) Scan(ctx context.Context, address string, start, end int64, cursor string) (Page, error) {
	canonical, err := BSCAddress(address)
	if err != nil {
		return Page{}, err
	}
	if canonical != address || start < 0 || end < start {
		return Page{}, fmt.Errorf("invalid BSC scan window or address")
	}
	var chain string
	if err := s.rpc(ctx, "eth_chainId", []any{}, &chain); err != nil {
		return Page{}, err
	}
	if chain != "0x38" {
		return Page{}, fmt.Errorf("BSC RPC must use mainnet chain ID 56")
	}
	_, head, headTime, err := s.block(ctx, "finalized")
	if err != nil {
		return Page{}, err
	}
	if head == int64(^uint64(0)>>1) {
		return Page{}, fmt.Errorf("invalid BSC head number")
	}
	if headTime < end {
		return Page{}, fmt.Errorf("BSC finalized head has not reached scan window")
	}
	// Lower bound includes all blocks with the same timestamp.
	low, high := int64(0), head+1
	for low < high {
		mid := low + (high-low)/2
		_, _, timestamp, err := s.block(ctx, fmt.Sprintf("0x%x", mid))
		if err != nil {
			return Page{}, err
		}
		if timestamp < start {
			low = mid + 1
		} else {
			high = mid
		}
	}
	from := low
	if cursor != "" {
		next, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || next < from || next > head {
			return Page{}, fmt.Errorf("invalid BSC scan cursor")
		}
		from = next
	}
	if from > head {
		return Page{}, nil
	}
	to := min(from+999, head)
	var logs []struct {
		Address         string   `json:"address"`
		Topics          []string `json:"topics"`
		Data            string   `json:"data"`
		TransactionHash string   `json:"transactionHash"`
		BlockNumber     string   `json:"blockNumber"`
		BlockHash       string   `json:"blockHash"`
		LogIndex        string   `json:"logIndex"`
		Removed         bool     `json:"removed"`
	}
	recipient := "0x" + strings.Repeat("0", 24) + address[2:]
	filter := map[string]any{"address": BSCUSDTContract, "fromBlock": fmt.Sprintf("0x%x", from), "toBlock": fmt.Sprintf("0x%x", to), "topics": []any{"0x" + transferTopic, nil, recipient}}
	if err := s.rpc(ctx, "eth_getLogs", []any{filter}, &logs); err != nil {
		return Page{}, err
	}
	page := Page{}
	blocks := make(map[int64]bscBlock)
	timestamps := make(map[int64]int64)
	for _, log := range logs {
		number, err := bscNumber(log.BlockNumber)
		if err != nil {
			return Page{}, err
		}
		index, err := bscNumber(log.LogIndex)
		if err != nil {
			return Page{}, err
		}
		if number < from || number > to || log.Removed || !bscHash(log.TransactionHash) || strings.ToLower(log.Address) != BSCUSDTContract || len(log.Topics) != 3 || strings.ToLower(log.Topics[0]) != "0x"+transferTopic || strings.ToLower(log.Topics[2]) != recipient {
			return Page{}, fmt.Errorf("BSC RPC returned an invalid transfer log")
		}
		block, ok := blocks[number]
		if !ok {
			var timestamp int64
			block, _, timestamp, err = s.block(ctx, log.BlockNumber)
			if err != nil {
				return Page{}, err
			}
			blocks[number], timestamps[number] = block, timestamp
		}
		if block.Hash != log.BlockHash {
			return Page{}, fmt.Errorf("BSC transfer block hash mismatch")
		}
		timestamp := timestamps[number]
		if timestamp < start || timestamp > end {
			continue
		}
		if len(log.Data) != 66 || !strings.HasPrefix(log.Data, "0x") {
			return Page{}, fmt.Errorf("invalid BSC Transfer amount")
		}
		data, err := hex.DecodeString(log.Data[2:])
		if err != nil {
			return Page{}, fmt.Errorf("invalid BSC Transfer amount")
		}
		value := new(big.Int).SetBytes(data)
		if value.Sign() == 0 {
			continue
		}
		amount, err := BSCMicroAmount(value.String())
		if err != nil {
			return Page{}, err
		}
		page.Transfers = append(page.Transfers, Transfer{Network: BSC, Asset: USDT, TransactionID: strings.ToLower(log.TransactionHash), EventIndex: int(index), Address: address, Amount: amount, RawAmount: value.String(), Timestamp: timestamp})
	}
	_, _, through, err := s.block(ctx, fmt.Sprintf("0x%x", to))
	if err != nil {
		return Page{}, err
	}
	if through <= end && to < head {
		page.Next = strconv.FormatInt(to+1, 10)
	}
	return page, nil
}

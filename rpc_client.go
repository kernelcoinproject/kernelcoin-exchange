package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

// CoinRPCClient communicates with cryptocurrency daemons (kernelcoind, litecoind, etc.)
type CoinRPCClient struct {
	url      string
	user     string
	password string
	coinName string
}

// KernelcoinRPCClient is an alias for backward compatibility
type KernelcoinRPCClient = CoinRPCClient

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSONRPCResponse represents JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error"`
	ID      int         `json:"id"`
}

// NewKernelcoinRPCClient creates an authenticated RPC client for Kernelcoin
func NewKernelcoinRPCClient(url, user, password string) *KernelcoinRPCClient {
	return &CoinRPCClient{
		url:      url,
		user:     user,
		password: password,
		coinName: "kernelcoin",
	}
}

// NewLitecoinRPCClient creates an authenticated RPC client for Litecoin
func NewLitecoinRPCClient(url, user, password string) *CoinRPCClient {
	return &CoinRPCClient{
		url:      url,
		user:     user,
		password: password,
		coinName: "litecoin",
	}
}

// call makes an authenticated RPC call
func (c *CoinRPCClient) call(method string, params []interface{}) (interface{}, error) {
	log.Printf("[RPC-%s] Calling method: %s with params: %v", strings.ToUpper(c.coinName), method, params)

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		log.Printf("[RPC-%s] ERROR: Failed to marshal request: %v", strings.ToUpper(c.coinName), err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("[RPC-%s] ERROR: Failed to create HTTP request: %v", strings.ToUpper(c.coinName), err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[RPC-%s] ERROR: RPC POST failed: %v", strings.ToUpper(c.coinName), err)
		return nil, fmt.Errorf("RPC POST failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[RPC-%s] ERROR: Failed to read response body: %v", strings.ToUpper(c.coinName), err)
		return nil, fmt.Errorf("RPC read error: %w", err)
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("[RPC-%s] ERROR: Failed to unmarshal response: %v", strings.ToUpper(c.coinName), err)
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	if response.Error != nil {
		log.Printf("[RPC-%s] ERROR: RPC returned error: %v", strings.ToUpper(c.coinName), response.Error)
		return nil, fmt.Errorf("RPC error: %v", response.Error)
	}

	log.Printf("[RPC-%s] SUCCESS: Result type: %T", strings.ToUpper(c.coinName), response.Result)
	return response.Result, nil
}

// GetNewAddress generates a new address from the RPC wallet
func (c *CoinRPCClient) GetNewAddress(label, addressType string) (string, error) {
	log.Printf("[RPC-%s] GetNewAddress: Generating new address with type '%s'", strings.ToUpper(c.coinName), addressType)
	result, err := c.call("getnewaddress", []interface{}{label, addressType})
	if err != nil {
		log.Printf("[RPC-%s] GetNewAddress ERROR: %v", strings.ToUpper(c.coinName), err)
		return "", err
	}

	addr, ok := result.(string)
	if !ok {
		log.Printf("[RPC-%s] GetNewAddress ERROR: unexpected result type: %T", strings.ToUpper(c.coinName), result)
		return "", fmt.Errorf("unexpected getnewaddress response type: %T", result)
	}

	log.Printf("[RPC-%s] GetNewAddress SUCCESS: %s", strings.ToUpper(c.coinName), addr)
	return addr, nil
}

// GetReceivedByAddress gets the amount received by a specific address
func (c *CoinRPCClient) GetReceivedByAddress(address string, minconf int) (float64, error) {
	log.Printf("[RPC-%s] GetReceivedByAddress: Checking address %s with minconf %d", strings.ToUpper(c.coinName), address, minconf)
	result, err := c.call("getreceivedbyaddress", []interface{}{address, minconf})
	if err != nil {
		log.Printf("[RPC-%s] GetReceivedByAddress ERROR: %v", strings.ToUpper(c.coinName), err)
		return 0, err
	}

	amount, ok := result.(float64)
	if !ok {
		log.Printf("[RPC-%s] GetReceivedByAddress ERROR: unexpected result type: %T", strings.ToUpper(c.coinName), result)
		return 0, fmt.Errorf("unexpected getreceivedbyaddress response type: %T", result)
	}

	log.Printf("[RPC-%s] GetReceivedByAddress SUCCESS: %f", strings.ToUpper(c.coinName), amount)
	return amount, nil
}

// ListReceivedByAddress gets transactions received by a specific address
func (c *CoinRPCClient) ListReceivedByAddress(minconf int, includeEmpty bool, includeWatchOnly bool) ([]interface{}, error) {
	log.Printf("[RPC-%s] ListReceivedByAddress: minconf %d", strings.ToUpper(c.coinName), minconf)
	result, err := c.call("listreceivedbyaddress", []interface{}{minconf, includeEmpty, includeWatchOnly})
	if err != nil {
		log.Printf("[RPC-%s] ListReceivedByAddress ERROR: %v", strings.ToUpper(c.coinName), err)
		return nil, err
	}

	txs, ok := result.([]interface{})
	if !ok {
		log.Printf("[RPC-%s] ListReceivedByAddress ERROR: unexpected result type: %T", strings.ToUpper(c.coinName), result)
		return nil, fmt.Errorf("unexpected listreceivedbyaddress response type: %T", result)
	}

	log.Printf("[RPC-%s] ListReceivedByAddress SUCCESS: %d entries", strings.ToUpper(c.coinName), len(txs))
	return txs, nil
}

// SendToAddress sends coins to an address with fees subtracted from amount
func (c *CoinRPCClient) SendToAddress(address string, amount float64) (string, error) {
	log.Printf("[RPC-%s] SendToAddress: sending %.8f to %s", strings.ToUpper(c.coinName), amount, address)
	result, err := c.call("sendtoaddress", []interface{}{address, amount, "", "", false, true})
	if err != nil {
		log.Printf("[RPC-%s] SendToAddress ERROR: %v", strings.ToUpper(c.coinName), err)
		return "", err
	}

	txid, ok := result.(string)
	if !ok {
		log.Printf("[RPC-%s] SendToAddress ERROR: unexpected result type: %T", strings.ToUpper(c.coinName), result)
		return "", fmt.Errorf("unexpected sendtoaddress response type: %T", result)
	}

	log.Printf("[RPC-%s] SendToAddress SUCCESS: %s", strings.ToUpper(c.coinName), txid)
	return txid, nil
}

// ElectrumClient handles Electrum binary calls
type ElectrumClient struct {
	binaryPath string
	withdrawFee float64
}

// NewElectrumClient creates a new Electrum client
func NewElectrumClient(binaryPath string, withdrawFee float64) *ElectrumClient {
	return &ElectrumClient{binaryPath: binaryPath, withdrawFee: withdrawFee}
}

// CreateNewAddress creates a new address using Electrum
func (e *ElectrumClient) CreateNewAddress() (string, error) {
	log.Printf("[ELECTRUM] CreateNewAddress: Generating new address")
	cmd := exec.Command(e.binaryPath, "createnewaddress")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[ELECTRUM] CreateNewAddress ERROR: %v", err)
		return "", err
	}
	address := strings.TrimSpace(string(output))
	log.Printf("[ELECTRUM] CreateNewAddress SUCCESS: %s", address)
	return address, nil
}

// GetAddressBalance gets the balance for an address using Electrum
func (e *ElectrumClient) GetAddressBalance(address string) (float64, float64, error) {
	log.Printf("[ELECTRUM] GetAddressBalance: Checking address %s", address)
	cmd := exec.Command(e.binaryPath, "getaddressbalance", address)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[ELECTRUM] GetAddressBalance ERROR: %v", err)
		return 0, 0, err
	}

	var result struct {
		Confirmed   string `json:"confirmed"`
		Unconfirmed string `json:"unconfirmed"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("[ELECTRUM] GetAddressBalance ERROR: Failed to parse JSON: %v", err)
		return 0, 0, err
	}

	confirmed, _ := strconv.ParseFloat(result.Confirmed, 64)
	unconfirmed, _ := strconv.ParseFloat(result.Unconfirmed, 64)

	log.Printf("[ELECTRUM] GetAddressBalance SUCCESS: confirmed=%.8f, unconfirmed=%.8f", confirmed, unconfirmed)
	return confirmed, unconfirmed, nil
}

// GetAddressHistory gets transaction history for an address using Electrum
func (e *ElectrumClient) GetAddressHistory(address string) ([]map[string]interface{}, error) {
	log.Printf("[ELECTRUM] GetAddressHistory: Checking address %s", address)
	cmd := exec.Command(e.binaryPath, "getaddresshistory", address)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[ELECTRUM] GetAddressHistory ERROR: %v", err)
		return nil, err
	}

	var result struct {
		Result []map[string]interface{} `json:"result"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("[ELECTRUM] GetAddressHistory ERROR: Failed to parse JSON: %v", err)
		return nil, err
	}

	log.Printf("[ELECTRUM] GetAddressHistory SUCCESS: %d transactions", len(result.Result))
	return result.Result, nil
}

// PayTo sends Litecoin to an address using Electrum with configurable fee
func (e *ElectrumClient) PayTo(address string, amount float64) (string, error) {
	fee := e.withdrawFee
	netAmount := amount - fee
	
	log.Printf("[ELECTRUM] PayTo: sending %.8f to %s (fee: %.8f, net: %.8f)", amount, address, fee, netAmount)
	
	if netAmount <= 0 {
		return "", fmt.Errorf("amount too small after fee deduction")
	}
	
	// Step 1: Create transaction hex
	amountStr := fmt.Sprintf("%.8f", netAmount)
	cmd := exec.Command(e.binaryPath, "payto", address, amountStr)
	log.Printf("[ELECTRUM] Step 1 - Creating transaction: %s %s %s %s", e.binaryPath, "payto", address, amountStr)
	hexOutput, err := cmd.Output()
	if err != nil {
		log.Printf("[ELECTRUM] PayTo Step 1 ERROR: %v", err)
		return "", err
	}
	hex := strings.TrimSpace(string(hexOutput))
	log.Printf("[ELECTRUM] Generated hex: %s", hex)
	
	// Step 2: Broadcast transaction
	cmd = exec.Command(e.binaryPath, "broadcast", hex)
	log.Printf("[ELECTRUM] Step 2 - Broadcasting: %s %s %s", e.binaryPath, "broadcast", hex)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[ELECTRUM] PayTo Step 2 ERROR: %v", err)
		return "", err
	}
	txid := strings.TrimSpace(string(output))
	log.Printf("[ELECTRUM] PayTo SUCCESS: %s (kept %.8f LTC as fee)", txid, fee)
	return txid, nil
}
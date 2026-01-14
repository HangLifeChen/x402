package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	x402 "github.com/coinbase/x402/go"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go"
)

// ChainType 区块链类型
type ChainType string

const (
	ChainEVM    ChainType = "evm"
	ChainSolana ChainType = "solana"
)

// ZkStashClientWithPayment 带x402支付的zkStash客户端
type ZkStashClientWithPayment struct {
	httpClient       *x402.X402Client
	walletAddr       string
	evmPrivateKey    string
	solanaPrivateKey string
	rpcURL           string
	chain            ChainType
}

type ZkStashInterface interface {
	GetMemories() ([]byte, error)
	CreateMemories(req *CreateMemoriesRequest) (*CreateMemoriesResponse, error)
}

func NewZkStashClientWithPayment(
	evmPrivateKey,
	rpcURL string,
	httpClient *x402.X402Client,
) (ZkStashInterface, error) {
	privateKey, err := crypto.HexToECDSA(evmPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	walletAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	return &ZkStashClientWithPayment{
		httpClient:    httpClient,
		walletAddr:    walletAddr,
		evmPrivateKey: evmPrivateKey,
		rpcURL:        rpcURL,
		chain:         ChainEVM,
	}, nil
}

// NewZkStashClientWithPaymentSolana 创建Solana链的客户端
func NewZkStashClientWithPaymentSolana(
	solanaPrivateKey string,
	rpcURL string,
	httpClient *x402.X402Client,
) (*ZkStashClientWithPayment, error) {
	// 从私钥创建Solana密钥对
	keyBytes, err := solana.PrivateKeyFromBase58(solanaPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Solana private key: %w", err)
	}

	return &ZkStashClientWithPayment{
		httpClient:       httpClient,
		walletAddr:       keyBytes.PublicKey().String(),
		solanaPrivateKey: solanaPrivateKey,
		rpcURL:           rpcURL,
		chain:            ChainSolana,
	}, nil
}

// GetMemories 获取记忆列表
func (c *ZkStashClientWithPayment) GetMemories() ([]byte, error) {
	// TODO: 实现获取记忆列表的逻辑
	// 需要使用 generateHeaders 生成签名头，然后发送HTTP请求
	return nil, fmt.Errorf("not implemented yet")
}

// CreateMemories 创建记忆 - 支持提取模式和直接模式
func (c *ZkStashClientWithPayment) CreateMemories(req *CreateMemoriesRequest) (*CreateMemoriesResponse, error) {
	// 验证请求参数
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if req.AgentId == "" {
		return nil, fmt.Errorf("agentId is required")
	}
	// 验证conversation和memories至少提供一个
	if len(req.Conversation) == 0 && len(req.Memories) == 0 {
		return nil, fmt.Errorf("either conversation or memories must be provided")
	}
	// 序列化请求体
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送HTTP请求
	respBytes, err := c.makeRequestWithResponse("POST", "/memories", string(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create memories: %w", err)
	}

	// 解析响应
	var response CreateMemoriesResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &response, nil
}

// makeRequestWithResponse 执行HTTP请求并返回响应体
func (c *ZkStashClientWithPayment) makeRequestWithResponse(method, path, body string) ([]byte, error) {
	httpClient := wrapHTTPClient(c.httpClient)

	fmt.Printf("Making request to: %s\n\n", c.rpcURL+path)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, c.rpcURL+path, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	headers, err := c.generateHeaders(method, path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to generate headers: %w", err)
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Response Status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("Response Body Length: %d bytes\n", len(bodyBytes))

	// Check if response body is empty
	if len(bodyBytes) == 0 {
		fmt.Println("⚠️  Response body is empty")
		// Extract payment response from headers if present
		paymentHeader := resp.Header.Get("PAYMENT-RESPONSE")
		if paymentHeader == "" {
			paymentHeader = resp.Header.Get("X-PAYMENT-RESPONSE")
		}

		if paymentHeader != "" {
			fmt.Println("\n💰 Payment Details:")
			settleResp, err := extractPaymentResponse(resp.Header)
			if err == nil {
				fmt.Printf("  Transaction: %s\n", settleResp.Transaction)
				fmt.Printf("  Network: %s\n", settleResp.Network)
				fmt.Printf("  Payer: %s\n", settleResp.Payer)
			} else {
				fmt.Printf("  Payment Header: %s\n", paymentHeader[:min(100, len(paymentHeader))])
			}
		}
		return bodyBytes, nil
	}

	// Try to decode response body as JSON
	var responseData interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		// If JSON decoding fails, print raw response
		fmt.Printf("⚠️  Failed to decode as JSON, showing raw response:\n")
		fmt.Printf("  %s\n", string(bodyBytes))
		return bodyBytes, fmt.Errorf("failed to decode response as JSON: %w", err)
	}

	fmt.Println("✅ Response body:")
	prettyJSON, _ := json.MarshalIndent(responseData, "  ", "  ")
	fmt.Printf("  %s\n", string(prettyJSON))

	// Extract payment response from headers if present
	paymentHeader := resp.Header.Get("PAYMENT-RESPONSE")
	if paymentHeader == "" {
		paymentHeader = resp.Header.Get("X-PAYMENT-RESPONSE")
	}

	if paymentHeader != "" {
		fmt.Println("\n💰 Payment Details:")
		settleResp, err := extractPaymentResponse(resp.Header)
		if err == nil {
			fmt.Printf("  Transaction: %s\n", settleResp.Transaction)
			fmt.Printf("  Network: %s\n", settleResp.Network)
			fmt.Printf("  Payer: %s\n", settleResp.Payer)
		}
	}

	return bodyBytes, nil
}

// generateHeaders 生成带签名的请求头
// method: HTTP方法 (GET, POST, PUT, DELETE等)
// path: 请求路径 (如 "/memories")
// body: 请求体 (可以是nil)
// 返回包含签名的headers map
func (c *ZkStashClientWithPayment) generateHeaders(method, path string, body interface{}) (map[string]string, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	// 1. 对body进行SHA256哈希
	bodyHash, err := hashBody(body)
	if err != nil {
		return nil, fmt.Errorf("failed to hash body: %w", err)
	}

	// 2. 创建规范消息: METHOD|PATH|BODY_HASH|TIMESTAMP
	message := fmt.Sprintf("%s|%s|%s|%s", strings.ToUpper(method), path, bodyHash, timestamp)

	var signature string
	var address string

	if c.chain == ChainEVM {
		// EVM实现
		signature, address, err = c.signEVMMessagesV2(message)
		if err != nil {
			return nil, fmt.Errorf("EVM signing failed: %w", err)
		}
	} else {
		// Solana实现
		signature, address, err = c.signSolanaMessages(message)
		if err != nil {
			return nil, fmt.Errorf("Solana signing failed: %w", err)
		}
	}

	return map[string]string{
		"x-wallet-address":   address,
		"x-wallet-timestamp": timestamp,
		"x-wallet-signature": signature,
		"Content-Type":       "application/json",
	}, nil
}

// signEVMMessages 使用EVM私钥签名消息 太底层了
func (c *ZkStashClientWithPayment) signEVMMessagesV1(message string) (signature, address string, err error) {
	// 从私钥创建私钥对象
	privateKey, err := crypto.HexToECDSA(c.evmPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("invalid private key: %w", err)
	}
	// 对消息进行哈希 (eth_signMessage使用keccak256)
	hash := crypto.Keccak256Hash([]byte(message))

	// 签名
	signatureBytes, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", "", fmt.Errorf("signing failed: %w", err)
	}
	// 调整签名格式 (v值从0/1调整为27/28)
	signatureBytes[64] += 27
	// 获取地址
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", "", fmt.Errorf("error casting public key to ECDSA")
	}
	address = crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	signature = hex.EncodeToString(signatureBytes)
	return signature, address, nil
}

func (c *ZkStashClientWithPayment) signEVMMessagesV2(message string) (signature, address string, err error) {
	privateKey, err := crypto.HexToECDSA(c.evmPrivateKey)
	if err != nil {
		return "", "", err
	}
	// ✅ 自动加 "\x19Ethereum Signed Message:\n"
	hash := accounts.TextHash([]byte(message))

	sig, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return "", "", err
	}

	// v 调整为 27 / 28（大多数 API 需要）
	sig[64] += 27

	pub := privateKey.Public().(*ecdsa.PublicKey)
	address = crypto.PubkeyToAddress(*pub).Hex()

	signature = hex.EncodeToString(sig)
	return
}

// signSolanaMessages 使用Solana私钥签名消息
func (c *ZkStashClientWithPayment) signSolanaMessages(message string) (signature, address string, err error) {
	// 从base58私钥创建密钥对
	keyBytes, err := solana.PrivateKeyFromBase58(c.solanaPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("invalid Solana private key: %w", err)
	}

	publicKey := keyBytes.PublicKey()
	address = publicKey.String()

	// 签名消息
	signatureBytes, err := keyBytes.Sign([]byte(message))
	if err != nil {
		return "", "", fmt.Errorf("signing failed: %w", err)
	}
	// 转换为base64
	signature = signatureBytes.String()
	return signature, address, nil
}

// makeRequest performs an HTTP GET request with payment handling
func (c *ZkStashClientWithPayment) makeRequest(client *x402.X402Client, path, method, body string) error {
	httpClient := wrapHTTPClient(client)

	fmt.Printf("Making request to: %s\n\n", c.rpcURL+path)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, c.rpcURL+path, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	headers, err := c.generateHeaders(method, path, body)
	if err != nil {
		return fmt.Errorf("failed to generate headers: %w", err)
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body first to check if it's empty
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Response Status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("Response Body Length: %d bytes\n", len(bodyBytes))

	// Check if response body is empty
	if len(bodyBytes) == 0 {
		fmt.Println("⚠️  Response body is empty")
		// Extract payment response from headers if present
		paymentHeader := resp.Header.Get("PAYMENT-RESPONSE")
		if paymentHeader == "" {
			paymentHeader = resp.Header.Get("X-PAYMENT-RESPONSE")
		}

		if paymentHeader != "" {
			fmt.Println("\n💰 Payment Details:")
			settleResp, err := extractPaymentResponse(resp.Header)
			if err == nil {
				fmt.Printf("  Transaction: %s\n", settleResp.Transaction)
				fmt.Printf("  Network: %s\n", settleResp.Network)
				fmt.Printf("  Payer: %s\n", settleResp.Payer)
			} else {
				fmt.Printf("  Payment Header: %s\n", paymentHeader[:min(100, len(paymentHeader))])
			}
		}
		return nil
	}

	// Try to decode response body as JSON
	var responseData interface{}
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		// If JSON decoding fails, print raw response
		fmt.Printf("⚠️  Failed to decode as JSON, showing raw response:\n")
		fmt.Printf("  %s\n", string(bodyBytes))
		return fmt.Errorf("failed to decode response as JSON: %w", err)
	}

	fmt.Println("✅ Response body:")
	prettyJSON, _ := json.MarshalIndent(responseData, "  ", "  ")
	fmt.Printf("  %s\n", string(prettyJSON))

	// Extract payment response from headers if present
	paymentHeader := resp.Header.Get("PAYMENT-RESPONSE")
	if paymentHeader == "" {
		paymentHeader = resp.Header.Get("X-PAYMENT-RESPONSE")
	}

	if paymentHeader != "" {
		fmt.Println("\n💰 Payment Details:")
		settleResp, err := extractPaymentResponse(resp.Header)
		if err == nil {
			fmt.Printf("  Transaction: %s\n", settleResp.Transaction)
			fmt.Printf("  Network: %s\n", settleResp.Network)
			fmt.Printf("  Payer: %s\n", settleResp.Payer)
		}
	}

	return nil
}

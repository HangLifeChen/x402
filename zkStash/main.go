package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// ZkStashClientWithPayment 带x402支付的zkStash客户端
type ZkStashClientWithPayment struct {
	httpClient    *http.Client
	walletAddr    string
	evmPrivateKey string
	rpcURL        string
}

// NewZkStashClientWithPayment 创建带x402支付的客户端
func NewZkStashClientWithPayment(evmPrivateKey string) (*ZkStashClientWithPayment, error) {
	// 移除0x前缀
	evmPrivateKey = strings.TrimPrefix(evmPrivateKey, "0x")

	// 解析私钥
	privateKey, err := crypto.HexToECDSA(evmPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	walletAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	return &ZkStashClientWithPayment{
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		walletAddr:    walletAddr,
		evmPrivateKey: evmPrivateKey,
		rpcURL:        "https://sepolia.base.org",
	}, nil
}

// generateSignature 生成请求签名
func (c *ZkStashClientWithPayment) generateSignature(method, path, body string, timestamp string) (string, error) {
	// 计算body的SHA256哈希
	bodyHash := sha256.Sum256([]byte(body))
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	// 构造规范消息: METHOD|PATH|BODY_HASH|TIMESTAMP
	message := fmt.Sprintf("%s|%s|%s|%s", strings.ToUpper(method), path, bodyHashHex, timestamp)

	// 使用私钥签名
	privateKey, err := crypto.HexToECDSA(c.evmPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// 使用以太坊签名方法 (signMessage)
	// 添加以太坊消息前缀: "\x19Ethereum Signed Message:\n" + len(message) + message
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	prefixedMessage := prefix + message

	hash := crypto.Keccak256Hash([]byte(prefixedMessage))
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %w", err)
	}

	// 转换为hex字符串
	signatureHex := hex.EncodeToString(signature)

	return signatureHex, nil
}

// doRequestWithPayment 执行带x402支付的HTTP请求
func (c *ZkStashClientWithPayment) doRequestWithPayment(method, path string, body interface{}) (*http.Response, error) {
	var bodyStr string
	var bodyReader io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyStr = string(bodyBytes)
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := "https://api.zkstash.ai" + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 生成时间戳
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// 分离路径和查询参数（签名时只使用路径部分）
	signPath := path
	if idx := strings.Index(path, "?"); idx != -1 {
		signPath = path[:idx]
	}

	// 生成签名
	signature, err := c.generateSignature(method, signPath, bodyStr, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signature: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-wallet-address", c.walletAddr)
	req.Header.Set("x-wallet-signature", signature)
	req.Header.Set("x-wallet-timestamp", timestamp)

	// 调试信息
	fmt.Printf("🔍 请求详情:\n")
	fmt.Printf("   URL: %s\n", url)
	fmt.Printf("   Method: %s\n", method)
	fmt.Printf("   Path (for signature): %s\n", signPath)
	fmt.Printf("   Wallet: %s\n", c.walletAddr)
	fmt.Printf("   Timestamp: %s\n", timestamp)
	if bodyStr != "" {
		fmt.Printf("   Body: %s\n", bodyStr)
	}
	fmt.Printf("   Signature: %s\n\n", signature)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	fmt.Printf("🔍 响应详情:\n")
	fmt.Printf("   Status: %d\n", resp.StatusCode)

	// 如果返回402，需要支付
	if resp.StatusCode == http.StatusPaymentRequired {
		resp.Body.Close()
		return c.handle402Response(method, path, body)
	}

	// 如果返回错误，读取响应体
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("   Body: %s\n\n", string(respBody))
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("   Success\n\n")
	return resp, nil
}

// handle402Response 处理402支付要求响应
func (c *ZkStashClientWithPayment) handle402Response(method, path string, body interface{}) (*http.Response, error) {
	var bodyStr string
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyStr = string(bodyBytes)
	}

	url := "https://api.zkstash.ai" + path

	// 生成时间戳
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// 分离路径和查询参数（签名时只使用路径部分）
	signPath := path
	if idx := strings.Index(path, "?"); idx != -1 {
		signPath = path[:idx]
	}

	// 生成签名
	signature, err := c.generateSignature(method, signPath, bodyStr, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signature: %w", err)
	}

	req, _ := http.NewRequest(method, url, bytes.NewReader([]byte(bodyStr)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-wallet-address", c.walletAddr)
	req.Header.Set("x-wallet-signature", signature)
	req.Header.Set("x-wallet-timestamp", timestamp)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// 读取402响应
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Printf("\n💰 收到402支付要求\n")
	fmt.Printf("   响应体: %s\n", string(respBody))

	var x402Resp struct {
		X402Version int              `json:"x402Version"`
		Error       string           `json:"error"`
		Accepts     []PaymentNetwork `json:"accepts"`
	}

	if err := json.Unmarshal(respBody, &x402Resp); err != nil {
		return nil, fmt.Errorf("解析402响应失败: %w", err)
	}

	fmt.Printf("   x402Version: %d\n", x402Resp.X402Version)
	fmt.Printf("   错误: %s\n", x402Resp.Error)
	fmt.Printf("   Accepts数量: %d\n", len(x402Resp.Accepts))
	fmt.Printf("   支持的网络:\n")

	// 选择网络（优先选择base-sepolia网络）
	var selectedNetworkIndex = -1
	for i, accept := range x402Resp.Accepts {
		payTo := accept.PayTo
		if payTo == "" {
			payTo = accept.Recipient
		}
		fmt.Printf("   [%d] %s - %s %s -> %s\n", i+1, accept.Network, accept.Amount, accept.Token, payTo)
		// 优先选择base-sepolia网络，如果没有则选择solana-devnet
		if accept.Network == "base-sepolia" && selectedNetworkIndex == -1 {
			selectedNetworkIndex = i
		} else if accept.Network == "solana-devnet" && selectedNetworkIndex == -1 {
			selectedNetworkIndex = i
		}
	}

	if selectedNetworkIndex == -1 {
		return nil, fmt.Errorf("没有找到支持的支付网络")
	}

	selectedNetwork := x402Resp.Accepts[selectedNetworkIndex]
	payTo := selectedNetwork.Recipient
	if payTo == "" {
		payTo = selectedNetwork.PayTo
	}
	fmt.Printf("\n✅ 选择网络: %s\n", selectedNetwork.Network)
	fmt.Printf("   金额: %s %s\n", selectedNetwork.Amount, selectedNetwork.Token)
	fmt.Printf("   收款地址: %s\n", payTo)

	// 执行支付
	paymentProof, err := c.executePayment(selectedNetwork)
	if err != nil {
		return nil, fmt.Errorf("支付失败: %w", err)
	}

	fmt.Printf("✅ 支付成功: %s\n\n", paymentProof)

	// 重试请求，带上支付证明
	var retryBodyBytes []byte
	if body != nil {
		retryBodyBytes, _ = json.Marshal(body)
	} else {
		retryBodyBytes = []byte("{}")
	}

	// 分离路径和查询参数（签名时只使用路径部分）
	retrySignPath := path
	if idx := strings.Index(path, "?"); idx != -1 {
		retrySignPath = path[:idx]
	}

	// 生成新的时间戳和签名
	retryTimestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	retrySignature, err := c.generateSignature(method, retrySignPath, string(retryBodyBytes), retryTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to generate retry signature: %w", err)
	}

	req, _ = http.NewRequest(method, url, bytes.NewReader(retryBodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-wallet-address", c.walletAddr)
	req.Header.Set("x-wallet-signature", retrySignature)
	req.Header.Set("x-wallet-timestamp", retryTimestamp)
	req.Header.Set("x-payment", paymentProof)

	return c.httpClient.Do(req)
}

// PaymentNetwork 支付网络信息
type PaymentNetwork struct {
	Network           string `json:"network"`
	Token             string `json:"token"`
	Amount            string `json:"amount"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	Recipient         string `json:"recipient"`
	PayTo             string `json:"payTo"` // 兼容性字段
}

// executePayment 执行链上支付
func (c *ZkStashClientWithPayment) executePayment(network PaymentNetwork) (string, error) {
	// 根据网络选择RPC URL
	var rpcURL string
	var chainID *big.Int
	switch network.Network {
	case "base":
		rpcURL = "https://mainnet.base.org"
		chainID = big.NewInt(8453)
	case "base-sepolia":
		rpcURL = "https://sepolia.base.org"
		chainID = big.NewInt(84532)
	default:
		return "", fmt.Errorf("不支持的网络: %s", network.Network)
	}

	// 连接到区块链
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return "", fmt.Errorf("连接RPC失败: %w", err)
	}
	defer client.Close()

	// 解析私钥（移除0x前缀）
	evmPrivateKey := strings.TrimPrefix(c.evmPrivateKey, "0x")
	privateKey, err := crypto.HexToECDSA(evmPrivateKey)
	if err != nil {
		return "", fmt.Errorf("解析私钥失败: %w", err)
	}

	// 获取nonce
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return "", fmt.Errorf("获取nonce失败: %w", err)
	}

	// 获取gas价格
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", fmt.Errorf("获取gas价格失败: %w", err)
	}

	// 解析收款地址
	payTo := network.PayTo
	if payTo == "" {
		payTo = network.Recipient
	}
	toAddress := common.HexToAddress(payTo)

	// 解析金额（USDC有6位小数）
	amount := big.NewInt(0)
	if network.MaxAmountRequired != "" {
		amount.SetString(network.MaxAmountRequired, 10)
	}

	// 根据网络选择USDC合约地址
	var usdcContractAddress common.Address
	switch network.Network {
	case "base":
		usdcContractAddress = common.HexToAddress("0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913") // Base主网
	case "base-sepolia":
		usdcContractAddress = common.HexToAddress("0x036CbD5384e5Dd998429035851e1a2e3") // Base Sepolia测试网
	default:
		return "", fmt.Errorf("不支持的网络: %s", network.Network)
	}

	// 构造USDC转账交易数据
	// transfer(address to, uint256 amount)
	transferMethodHash := crypto.Keccak256Hash([]byte("transfer(address,uint256)"))
	transferMethodID := transferMethodHash.Bytes()[:4]
	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	data := append(transferMethodID, paddedAddress...)
	data = append(data, paddedAmount...)

	// 构造交易
	tx := types.NewTransaction(
		nonce,
		usdcContractAddress,
		big.NewInt(0), // value为0，因为是ERC20转账
		200000,        // gas limit
		gasPrice,
		data,
	)

	// 签名交易
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return "", fmt.Errorf("签名交易失败: %w", err)
	}

	// 发送交易
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("发送交易失败: %w", err)
	}

	fmt.Printf("   交易哈希: %s\n", signedTx.Hash().Hex())
	fmt.Printf("   等待交易确认...\n")

	// 等待交易确认（最多等待60秒）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for {
		receipt, err := client.TransactionReceipt(ctx, signedTx.Hash())
		if err != nil {
			if ctx.Err() != nil {
				return "", fmt.Errorf("等待交易确认超时: %w", err)
			}
			time.Sleep(2 * time.Second)
			continue
		}

		if receipt != nil && receipt.Status == types.ReceiptStatusSuccessful {
			fmt.Printf("   交易确认成功，区块: %d\n", receipt.BlockNumber.Int64())
			break
		} else if receipt != nil && receipt.Status == types.ReceiptStatusFailed {
			return "", fmt.Errorf("交易失败")
		}

		time.Sleep(2 * time.Second)
	}

	// 构造x402支付证明
	paymentPayload := map[string]interface{}{
		"scheme":  "exact",
		"network": network.Network,
		"asset":   "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913", // Base USDC合约地址
		"amount":  network.MaxAmountRequired,
		"payTo":   payTo,
		"tx":      signedTx.Hash().Hex(),
	}

	// 使用确定性JSON序列化（按字母顺序排序键）
	paymentJSON, _ := json.Marshal(paymentPayload)
	paymentProof := base64.StdEncoding.EncodeToString(paymentJSON)

	fmt.Printf("   支付证明（base64）: %s\n", paymentProof)
	fmt.Printf("   支付证明（解码）: %s\n\n", string(paymentJSON))

	return paymentProof, nil
}

// CreateMemoryRequest 创建记忆请求
type CreateMemoryRequest struct {
	AgentId      string                `json:"agentId"`
	SubjectId    string                `json:"subjectId,omitempty"`
	Conversation []ConversationMessage `json:"conversation,omitempty"`
	Memories     []DirectMemory        `json:"memories,omitempty"`
	ThreadId     string                `json:"threadId,omitempty"`
	Schemas      []string              `json:"schemas,omitempty"`
	TTL          string                `json:"ttl,omitempty"`
	ExpiresAt    int64                 `json:"expiresAt,omitempty"`
}

// ConversationMessage 对话消息
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	ID      string `json:"id,omitempty"`
}

// DirectMemory 直接记忆
type DirectMemory struct {
	Kind      string                 `json:"kind"`
	Data      map[string]interface{} `json:"data"`
	ID        string                 `json:"id,omitempty"`
	TTL       string                 `json:"ttl,omitempty"`
	ExpiresAt int64                  `json:"expiresAt,omitempty"`
}

// CreateMemoryResponse 创建记忆响应
type CreateMemoryResponse struct {
	Success bool     `json:"success"`
	Created []Memory `json:"created"`
	Updated []Memory `json:"updated"`
}

// Memory 记忆
type Memory struct {
	ID       string                 `json:"id"`
	Kind     string                 `json:"kind"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateMemory 创建记忆
func (c *ZkStashClientWithPayment) CreateMemory(req CreateMemoryRequest) (*CreateMemoryResponse, error) {
	resp, err := c.doRequestWithPayment("POST", "/memories", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result CreateMemoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// SearchMemoryRequest 搜索记忆请求参数
type SearchMemoryRequest struct {
	Query     string
	AgentId   string
	SubjectId string
	ThreadId  string
	Kind      string
	Tags      string
	Limit     int
	Mode      string
	Scope     string
}

// SearchMemoryResponse 搜索记忆响应
type SearchMemoryResponse struct {
	Success    bool     `json:"success"`
	Memories   []Memory `json:"memories"`
	SearchedAt string   `json:"searchedAt"`
}

// SearchMemories 搜索记忆
func (c *ZkStashClientWithPayment) SearchMemories(req SearchMemoryRequest) (*SearchMemoryResponse, error) {
	// 构造查询参数
	params := []string{fmt.Sprintf("query=%s", req.Query)}
	if req.AgentId != "" {
		params = append(params, fmt.Sprintf("agentId=%s", req.AgentId))
	}
	if req.SubjectId != "" {
		params = append(params, fmt.Sprintf("subjectId=%s", req.SubjectId))
	}
	if req.ThreadId != "" {
		params = append(params, fmt.Sprintf("threadId=%s", req.ThreadId))
	}
	if req.Kind != "" {
		params = append(params, fmt.Sprintf("kind=%s", req.Kind))
	}
	if req.Tags != "" {
		params = append(params, fmt.Sprintf("tags=%s", req.Tags))
	}
	if req.Limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", req.Limit))
	}
	if req.Mode != "" {
		params = append(params, fmt.Sprintf("mode=%s", req.Mode))
	}
	if req.Scope != "" {
		params = append(params, fmt.Sprintf("scope=%s", req.Scope))
	}

	path := "/memories/search?" + strings.Join(params, "&")

	resp, err := c.doRequestWithPayment("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result SearchMemoryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, using environment variables")
	}

	evmPrivateKey := os.Getenv("EVM_PRIVATE_KEY")
	if evmPrivateKey == "" {
		fmt.Println("❌ 请设置环境变量 EVM_PRIVATE_KEY")
		fmt.Println("   示例: export EVM_PRIVATE_KEY=0x...")
		os.Exit(1)
	}

	// 创建带x402支付的客户端
	client, err := NewZkStashClientWithPayment(evmPrivateKey)
	if err != nil {
		fmt.Printf("❌ 创建客户端失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ zkStash客户端创建成功（带x402支付）\n")
	fmt.Printf("   钱包地址: %s\n", client.walletAddr)
	fmt.Printf("   网络: Base Sepolia (测试网)\n\n")

	// 演示1: 创建记忆（提取模式）
	fmt.Println("=== 演示1: 创建记忆（提取模式）===")
	createReq := CreateMemoryRequest{
		AgentId:   "demo-agent",
		SubjectId: "user-001",
		Conversation: []ConversationMessage{
			{
				ID:      "msg_001",
				Role:    "user",
				Content: "我叫张三，今年25岁，是一名软件工程师",
			},
			{
				ID:      "msg_002",
				Role:    "assistant",
				Content: "好的，我已经记住了你的信息",
			},
		},
	}

	createResp, err := client.CreateMemory(createReq)
	if err != nil {
		fmt.Printf("❌ 创建记忆失败: %v\n", err)
	} else {
		fmt.Printf("✅ 创建记忆成功\n")
		fmt.Printf("   创建了 %d 条记忆\n", len(createResp.Created))
		for _, mem := range createResp.Created {
			fmt.Printf("   - ID: %s, Kind: %s\n", mem.ID, mem.Kind)
		}
	}

	// 等待一下，让索引更新
	time.Sleep(2 * time.Second)

	// 演示2: 搜索记忆
	fmt.Println("\n=== 演示2: 搜索记忆 ===")
	searchReq := SearchMemoryRequest{
		Query:     "张三",
		AgentId:   "demo-agent",
		SubjectId: "user-001",
		Limit:     5,
	}

	searchResp, err := client.SearchMemories(searchReq)
	if err != nil {
		fmt.Printf("❌ 搜索记忆失败: %v\n", err)
	} else {
		fmt.Printf("✅ 搜索成功，找到 %d 条记忆\n", len(searchResp.Memories))
		for i, mem := range searchResp.Memories {
			fmt.Printf("\n   记忆 %d:\n", i+1)
			fmt.Printf("   - ID: %s\n", mem.ID)
			fmt.Printf("   - Kind: %s\n", mem.Kind)
			if mem.Data != nil {
				fmt.Printf("   - Data: %v\n", mem.Data)
			}
		}
	}

	fmt.Println("\n=== 演示完成 ===")
}

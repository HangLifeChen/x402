# zkStash Go Demo (带x402支付)

这是一个使用 Go 语言实现的 zkStash API 客户端示例，演示如何上传和拉取记忆，并集成 x402 协议进行支付。

## 功能特性

- ✅ x402 协议自动支付（Base Sepolia 测试网）
- ✅ 创建记忆（提取模式）
- ✅ 创建记忆（直接模式）
- ✅ 搜索记忆
- ✅ 多租户隔离（subjectId）
- ✅ 支持自定义记忆类型
- ✅ 自动处理 402 支付要求

## 前置要求

1. Go 1.19 或更高版本
2. 一个有效的以太坊私钥（Base Sepolia 测试网）
3. Base Sepolia 测试网 ETH（用于支付 gas）
4. zkStash API 访问权限

## 安装依赖

```bash
cd zkStash
go mod tidy
```

## 配置

1. 复制环境变量模板：
```bash
cp .env.example .env
```

2. 编辑 `.env` 文件，设置你的私钥：
```env
EVM_PRIVATE_KEY=0x你的私钥
```

## 运行 Demo

```bash
go run main.go
```

## Demo 演示内容

### 1. 创建记忆（提取模式）

自动从对话中提取结构化记忆：

```go
createReq := CreateMemoryRequest{
    AgentId: "demo-agent",
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
```

### 2. 创建记忆（直接模式）

直接存储结构化数据：

```go
directReq := CreateMemoryRequest{
    AgentId: "demo-agent",
    SubjectId: "user-001",
    Memories: []DirectMemory{
        {
            Kind: "UserProfile",
            Data: map[string]interface{}{
                "name":   "张三",
                "age":    25,
                "job":    "软件工程师",
                "hobby":  "编程",
            },
        },
    },
}
```

### 3. 搜索记忆

使用自然语言搜索记忆：

```go
searchReq := SearchMemoryRequest{
    Query:     "张三",
    AgentId:   "demo-agent",
    SubjectId: "user-001",
    Limit:     5,
}
```

## x402 支付流程

当 API 返回 402 Payment Required 时，客户端会自动：

1. 解析支付要求（支持的网络、金额、收款地址）
2. 选择 Base Sepolia 测试网
3. 在链上执行支付交易
4. 构造 x402 支付证明
5. 重试请求，带上支付证明

### 支付流程图

```
客户端请求 zkStash API
    ↓
返回 402 Payment Required
    ↓
解析支付要求
    ↓
选择 Base Sepolia 网络
    ↓
执行链上支付
    ↓
构造 x402 支付证明
    ↓
重试请求（带 x-payment header）
    ↓
成功获取数据
```

## 多租户隔离

使用 `subjectId` 实现数据隔离：

```go
// 用户A的记忆
CreateMemoryRequest{
    AgentId: "customer-support-bot",
    SubjectId: "tenant-123",  // 公司A
    ...
}

// 用户B的记忆
CreateMemoryRequest{
    AgentId: "customer-support-bot",
    SubjectId: "tenant-456",  // 公司B
    ...
}
```

## 注意事项

1. **私钥安全**：不要将私钥提交到代码仓库
2. **测试网 ETH**：确保你的钱包有足够的 Base Sepolia ETH 支付 gas
3. **时间同步**：确保服务器时间准确，签名有效期2分钟
4. **API 限制**：注意 API 调用频率限制
5. **支付金额**：每次 API 调用可能需要支付少量费用

## 错误处理

Demo 包含基本的错误处理，常见错误：

- `401 Unauthorized`: 签名无效或时间戳过期
- `402 Payment Required`: 需要支付信用（会自动处理）
- `400 Bad Request`: 请求参数错误
- `500 Internal Error`: 服务器错误

## 获取测试网 ETH

1. 访问 Base Sepolia 水龙头：https://sepoliafaucet.com/
2. 输入你的钱包地址
3. 等待 ETH 到账

## 下一步

1. 集成到你的产品中
2. 实现记忆共享（Grants）
3. 添加能力证明（Attestations）
4. 实现自主代理自我融资

## 相关文档

- [zkStash API 文档](zkStashAPI.md)
- [x402 协议](https://www.x402.org/)
- [Base Sepolia 文档](https://docs.base.org/using-base/testnet)

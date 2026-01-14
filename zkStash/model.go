package main

// ConversationMessage 对话消息对象
type ConversationMessage struct {
	ID      string `json:"id,omitempty"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DirectMemory 直接记忆对象
type DirectMemory struct {
	Kind      string                 `json:"kind"`
	Data      map[string]interface{} `json:"data"`
	ID        string                 `json:"id,omitempty"`
	TTL       string                 `json:"ttl,omitempty"`
	ExpiresAt int64                  `json:"expiresAt,omitempty"`
}

// CreateMemoriesRequest 创建记忆请求
type CreateMemoriesRequest struct {
	AgentId      string                `json:"agentId"`
	SubjectId    string                `json:"subjectId,omitempty"`
	Conversation []ConversationMessage `json:"conversation,omitempty"`
	Memories     []DirectMemory        `json:"memories,omitempty"`
	ThreadId     string                `json:"threadId,omitempty"`
	Schemas      []string              `json:"schemas,omitempty"`
	TTL          string                `json:"ttl,omitempty"`
	ExpiresAt    int64                 `json:"expiresAt,omitempty"`
}

// MemoryMetadata 记忆元数据
type MemoryMetadata struct {
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata"`
}

// CreateMemoriesResponse 创建记忆响应
type CreateMemoriesResponse struct {
	Success bool             `json:"success"`
	Created []MemoryMetadata `json:"created"`
	Updated []MemoryMetadata `json:"updated"`
}

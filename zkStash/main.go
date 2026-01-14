package main

import (
	"fmt"
	"os"

	x402 "github.com/coinbase/x402/go"
	"github.com/joho/godotenv"
)

func main() { // Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, using environment variables")
	}
	pattern := "mechanism-helper-registration"
	if len(os.Args) > 1 {
		pattern = os.Args[1]
	}
	fmt.Printf("\nRunning example: %s\n\n", pattern)

	// Get configuration
	evmPrivateKey := os.Getenv("EVM_PRIVATE_KEY")
	if evmPrivateKey == "" {
		fmt.Println("❌ EVM_PRIVATE_KEY environment variable is required")
		os.Exit(1)
	}
	svmPrivateKey := os.Getenv("SVM_PRIVATE_KEY")

	url := os.Getenv("SERVER_URL")
	if url == "" {
		url = "https://api.zkstash.ai"
	}

	// Create client based on pattern
	var client *x402.X402Client
	var err error

	switch pattern {
	case "builder-pattern":
		client, err = createBuilderPatternClient(evmPrivateKey, svmPrivateKey)
	case "mechanism-helper-registration":
		client, err = createMechanismHelperRegistrationClient(evmPrivateKey, svmPrivateKey)
	default:
		fmt.Printf("❌ Unknown pattern: %s\n", pattern)
		fmt.Println("Available patterns: builder-pattern, mechanism-helper-registration")
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("❌ Failed to create client: %v\n", err)
		os.Exit(1)
	}

	ZkStashClientWithPayment, err := NewZkStashClientWithPayment(evmPrivateKey, url, client)
	if err != nil {
		fmt.Printf("❌ Failed to create ZkStashClientWithPayment: %v\n", err)
		os.Exit(1)
	}
	// Make the request

	resp, err := ZkStashClientWithPayment.CreateMemories(
		&CreateMemoriesRequest{
			AgentId: "agent-007",
			Conversation: []ConversationMessage{
				{
					Role:    "user",
					Content: "My favorite color is blue.",
				},
				{
					Role:    "assistant",
					Content: "Noted.",
				},
			},
		})
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Response: %+v\n", resp)
}

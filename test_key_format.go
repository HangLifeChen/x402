package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
)

func main() {
	// 直接从 .env 文件读取
	keySecret := "c4KyNT9OwYiMOXxYzEy/lekTj8+QJv7dDeO1psK6rctkk4NUC4JgNs8L1K7xoO7TJhI/7HTwPREkxGT+0IFbg=="

	// Decode base64 key
	decoded, err := base64.StdEncoding.DecodeString(keySecret)
	if err != nil {
		log.Fatalf("Failed to decode base64: %v", err)
	}

	fmt.Printf("Key length: %d bytes\n", len(decoded))
	fmt.Printf("Expected: 64 bytes for Ed25519\n")

	if len(decoded) != 64 {
		fmt.Printf("\n❌ ERROR: Invalid key length!\n")
		fmt.Printf("Ed25519 keys must be exactly 64 bytes (32 bytes seed + 32 bytes public key)\n")
		fmt.Printf("Your key has %d bytes - it's likely truncated or corrupted.\n", len(decoded))
		fmt.Printf("\nPlease regenerate your API key from Coinbase CDP dashboard:\n")
		fmt.Printf("https://portal.cdp.coinbase.com/settings/api-keys\n")
		os.Exit(1)
	}

	fmt.Println("\n✓ Key format is correct!")
}

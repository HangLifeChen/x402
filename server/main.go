package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	x402 "github.com/coinbase/x402/go"
	x402http "github.com/coinbase/x402/go/http"
	ginmw "github.com/coinbase/x402/go/http/gin"
	evm "github.com/coinbase/x402/go/mechanisms/evm/exact/server"
	svm "github.com/coinbase/x402/go/mechanisms/svm/exact/server"
	ginfw "github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

const (
	DefaultPort = "4021"
)

func main() {
	godotenv.Load()

	evmAddress := os.Getenv("EVM_PAYEE_ADDRESS")
	if evmAddress == "" {
		fmt.Println("❌ EVM_PAYEE_ADDRESS environment variable is required")
		os.Exit(1)
	}

	svmAddress := os.Getenv("SVM_PAYEE_ADDRESS")
	if svmAddress == "" {
		fmt.Println("❌ SVM_PAYEE_ADDRESS environment variable is required")
		os.Exit(1)
	}

	facilitatorURL := os.Getenv("FACILITATOR_URL")
	if facilitatorURL == "" {
		fmt.Println("❌ FACILITATOR_URL environment variable is required")
		fmt.Println("   Example: https://x402.org/facilitator")
		os.Exit(1)
	}
	cdpAPIKeyID := os.Getenv("CDP_API_KEY_ID")
	if cdpAPIKeyID == "" {
		fmt.Println("❌ CDP_API_KEY_ID environment variable is required")
		os.Exit(1)
	}
	cdpAPIKeySecret := os.Getenv("CDP_API_KEY_SECRET")
	if cdpAPIKeySecret == "" {
		fmt.Println("❌ CDP_API_KEY_SECRET environment variable is required")
		os.Exit(1)
	}
	// Network configuration - Base Sepolia testnet
	// evmNetwork := x402.Network("eip155:84532")
	// svmNetwork := x402.Network("solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1")
	evmNetwork2 := x402.Network("eip155:8453")
	svmNetwork2 := x402.Network("solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp")
	fmt.Printf("🚀 Starting Gin x402 server...\n")
	fmt.Printf("   EVM Payee address: %s\n", evmAddress)
	fmt.Printf("   SVM Payee address: %s\n", svmAddress)
	fmt.Printf("   EVM Network: %s\n", evmNetwork2)
	fmt.Printf("   SVM Network: %s\n", svmNetwork2)
	fmt.Printf("   Facilitator: %s\n", facilitatorURL)
	fmt.Printf("   CDP API Key ID: %s\n", cdpAPIKeyID)
	fmt.Printf("   CDP API Key Secret: %s\n", cdpAPIKeySecret)

	// Create Gin router
	r := ginfw.Default()

	// Create HTTP facilitator client
	facilitatorClient := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{
		URL: facilitatorURL,
	})

	// 创建CDP认证提供者
	// cdpAuthProvider := &CDPAuthProvider{
	// 	APIKeyID:     cdpAPIKeyID,
	// 	APIKeySecret: cdpAPIKeySecret,
	// }

	// facilitatorClient := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{
	// 	URL:          facilitatorURL,
	// 	AuthProvider: cdpAuthProvider,
	// 	Timeout:      30 * time.Second,
	// })

	/**
	 * Configure x402 payment middleware
	 *
	 * This middleware protects specific routes with payment requirements.
	 * When a client accesses a protected route without payment, they receive
	 * a 402 Payment Required response with payment details.
	 */
	routes := x402http.RoutesConfig{
		"GET /weather": {
			Accepts: x402http.PaymentOptions{
				{
					Scheme:  "exact",
					Price:   "$0.001",
					Network: "eip155:84532",
					PayTo:   evmAddress,
				},
				{
					Scheme:  "exact",
					Price:   "$0.001",
					Network: "solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1",
					PayTo:   svmAddress,
				},
			},
			Description: "Get weather data for a city",
			MimeType:    "application/json",
		},
		"GET /zkStash": {
			Accepts: x402http.PaymentOptions{
				{
					Scheme:  "exact",
					Price:   "$0.001",
					Network: "eip155:8453",
					PayTo:   evmAddress,
				},
				{
					Scheme:  "exact",
					Price:   "$0.001",
					Network: "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",
					PayTo:   svmAddress,
				},
			},
			Description: "Get weather data for a city",
			MimeType:    "application/json",
		},
	}

	// Apply x402 payment middleware
	r.Use(ginmw.X402Payment(ginmw.Config{
		Routes:      routes,
		Facilitator: facilitatorClient,
		Schemes: []ginmw.SchemeConfig{
			// {Network: "eip155:84532", Server: evm.NewExactEvmScheme()},
			// {Network: "solana:EtWTRABZaYq6iMfeYKouRu166VU2xqa1", Server: svm.NewExactSvmScheme()},
			{Network: "eip155:8453", Server: evm.NewExactEvmScheme()},
			{Network: "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp", Server: svm.NewExactSvmScheme()},
		},
		Timeout: 30 * time.Second,
	}))

	/**
	 * Protected endpoint - requires $0.001 USDC payment
	 *
	 * Clients must provide a valid x402 payment to access this endpoint.
	 * The payment is verified and settled before the endpoint handler runs.
	 */
	r.GET("/weather", func(c *ginfw.Context) {
		city := c.DefaultQuery("city", "San Francisco")

		weatherData := map[string]map[string]interface{}{
			"San Francisco": {"weather": "foggy", "temperature": 60},
			"New York":      {"weather": "cloudy", "temperature": 55},
			"London":        {"weather": "rainy", "temperature": 50},
			"Tokyo":         {"weather": "clear", "temperature": 65},
		}

		data, exists := weatherData[city]
		if !exists {
			data = map[string]interface{}{"weather": "sunny", "temperature": 70}
		}

		c.JSON(http.StatusOK, ginfw.H{
			"city":        city,
			"weather":     data["weather"],
			"temperature": data["temperature"],
			"timestamp":   time.Now().Format(time.RFC3339),
		})
	})

	r.GET("/zkStash", func(c *ginfw.Context) {
		c.JSON(http.StatusOK, ginfw.H{
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	/**
	 * Health check endpoint - no payment required
	 *
	 * This endpoint is not protected by x402 middleware.
	 */
	r.GET("/health", func(c *ginfw.Context) {
		c.JSON(http.StatusOK, ginfw.H{
			"status":  "ok",
			"version": "2.0.0",
		})
	})

	fmt.Printf("   Server listening on http://localhost:%s\n\n", DefaultPort)

	if err := r.Run(":" + DefaultPort); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"fmt"

	"github.com/coinbase/cdp-sdk/go/auth"
	x402http "github.com/coinbase/x402/go/http"
)

// CDPAuthProvider implements AuthProvider for Coinbase CDP API
type CDPAuthProvider struct {
	APIKeyID     string
	APIKeySecret string
}

func (a *CDPAuthProvider) GetAuthHeaders(ctx context.Context) (x402http.AuthHeaders, error) {
	// Generate JWT for /supported endpoint
	supportedJWT, err := generateCDPJWT(a.APIKeyID, a.APIKeySecret, "GET", "api.cdp.coinbase.com", "/platform/v2/x402/supported")
	if err != nil {
		return x402http.AuthHeaders{}, fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Generate JWT for /verify endpoint
	verifyJWT, err := generateCDPJWT(a.APIKeyID, a.APIKeySecret, "POST", "api.cdp.coinbase.com", "/platform/v2/x402/verify")
	if err != nil {
		return x402http.AuthHeaders{}, fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Generate JWT for /settle endpoint
	settleJWT, err := generateCDPJWT(a.APIKeyID, a.APIKeySecret, "POST", "api.cdp.coinbase.com", "/platform/v2/x402/settle")
	if err != nil {
		return x402http.AuthHeaders{}, fmt.Errorf("failed to generate JWT: %w", err)
	}

	return x402http.AuthHeaders{
		Supported: map[string]string{
			"Authorization": "Bearer " + supportedJWT,
		},
		Verify: map[string]string{
			"Authorization": "Bearer " + verifyJWT,
		},
		Settle: map[string]string{
			"Authorization": "Bearer " + settleJWT,
		},
	}, nil
}

// generateCDPJWT generates a JWT token for CDP API authentication
func generateCDPJWT(keyID, keySecret, method, host, path string) (string, error) {
	jwt, err := auth.GenerateJWT(auth.JwtOptions{
		KeyID:         keyID,
		KeySecret:     keySecret,
		RequestMethod: method,
		RequestHost:   host,
		RequestPath:   path,
		ExpiresIn:     120, // 2 minutes
	})
	if err != nil {
		return "", err
	}
	return jwt, nil
}

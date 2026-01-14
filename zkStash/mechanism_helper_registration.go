package main

import (
	x402 "github.com/coinbase/x402/go"
	evm "github.com/coinbase/x402/go/mechanisms/evm/exact/client"
	evmv1 "github.com/coinbase/x402/go/mechanisms/evm/exact/v1/client"
	svm "github.com/coinbase/x402/go/mechanisms/svm/exact/client"
	svmv1 "github.com/coinbase/x402/go/mechanisms/svm/exact/v1/client"
	evmsigners "github.com/coinbase/x402/go/signers/evm"
	svmsigners "github.com/coinbase/x402/go/signers/svm"
)

/**
 * Mechanism Helper Registration Client
 *
 * This demonstrates a convenient pattern using mechanism helpers with wildcard
 * network registration for clean, readable client configuration.
 *
 * This approach is simpler than the builder pattern when you want to register
 * all networks of a particular type with the same signer.
 */

func createMechanismHelperRegistrationClient(evmPrivateKey, svmPrivateKey string) (*x402.X402Client, error) {
	// Create signers from private keys
	evmSigner, err := evmsigners.NewClientSignerFromPrivateKey(evmPrivateKey)
	if err != nil {
		return nil, err
	}

	// Start with a new client
	client := x402.Newx402Client()

	// Register EVM scheme for all EVM networks using wildcard
	// This registers:
	// - eip155:* (all EVM networks in v2)
	client.Register("eip155:*", evm.NewExactEvmScheme(evmSigner))

	// ✅ V1 fallback (zkStash 目前会返回 x402Version=1 的 payment requirements)
	// 常见网络标识：base-mainnet/base-sepolia/base（不同服务实现可能会有差异）
	client.RegisterV1("base-mainnet", evmv1.NewExactEvmSchemeV1(evmSigner))
	client.RegisterV1("base", evmv1.NewExactEvmSchemeV1(evmSigner))

	// Register SVM scheme if key is provided
	if svmPrivateKey != "" {
		svmSigner, err := svmsigners.NewClientSignerFromPrivateKey(svmPrivateKey)
		if err != nil {
			return nil, err
		}

		// Register for all Solana networks using wildcard
		// This registers:
		// - solana:* (all Solana networks in v2)
		client.Register("solana:*", svm.NewExactSvmScheme(svmSigner))

		// ✅ V1 fallback（同上：solana-mainnet/solana-devnet/solana）
		client.RegisterV1("solana-mainnet", svmv1.NewExactSvmSchemeV1(svmSigner))
		client.RegisterV1("solana", svmv1.NewExactSvmSchemeV1(svmSigner))
	}

	// The fluent API allows chaining for clean code:
	// client := x402.Newx402Client().
	//     Register("eip155:*", evm.NewExactEvmScheme(evmSigner)).
	//     Register("solana:*", svm.NewExactSvmScheme(svmSigner))

	return client, nil
}

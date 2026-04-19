//go:build integration

package namecoin

import (
	"context"
	"encoding/hex"
	"testing"
	"time"
)

// Live tests that hit a real ElectrumX server. Gated behind the
// "integration" build tag because they require network access and the
// Namecoin ecosystem's public ElectrumX servers to be available.
//
// Run with: go test -tags=integration ./namecoin -run Integration -v
//
// Known-good fixtures captured against electrumx.testls.space:50002:
//   testls.bit     → 460c25e682fda7832b52d1f22d3d22b3176d972f60dcdc3212ed8c92ef85065c
//   m@testls.bit   → 6cdebccabda1dfa058ab85352a79509b592b2bdfa0370325e28ec1cb4f18667d

const (
	testlsRootPubkey = "460c25e682fda7832b52d1f22d3d22b3176d972f60dcdc3212ed8c92ef85065c"
	testlsMPubkey    = "6cdebccabda1dfa058ab85352a79509b592b2bdfa0370325e28ec1cb4f18667d"
)

func TestIntegration_ResolveTestlsBit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pp, err := QueryIdentifier(ctx, "testls.bit")
	if err != nil {
		t.Fatalf("QueryIdentifier(testls.bit): %v", err)
	}
	if got := hex.EncodeToString(pp.PublicKey[:]); got != testlsRootPubkey {
		t.Errorf("testls.bit pubkey = %s, want %s", got, testlsRootPubkey)
	}
}

func TestIntegration_ResolveAtTestlsBit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pp, err := QueryIdentifier(ctx, "m@testls.bit")
	if err != nil {
		t.Fatalf("QueryIdentifier(m@testls.bit): %v", err)
	}
	if got := hex.EncodeToString(pp.PublicKey[:]); got != testlsMPubkey {
		t.Errorf("m@testls.bit pubkey = %s, want %s", got, testlsMPubkey)
	}
}

// --- WSS (WebSocket over TLS) integration tests -----------------------
//
// These hit the same ElectrumX operators as the TCP tests above, but
// over the WebSocket endpoints (ports 500xx+2 on testls, 570xx+2 on
// nmc2.bitcoins.sk). Same pinned certs, same JSON-RPC protocol, just
// different framing. The assertions match the TCP path because the
// Namecoin blockchain is the source of truth.

func TestIntegration_ResolveTestlsBit_WSS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewElectrumClient()
	result, err := client.NameShowWithFallback(ctx, "d/testls", DefaultElectrumXServersWSS)
	if err != nil {
		t.Fatalf("NameShowWithFallback(d/testls, WSS): %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	pk, _, err := extractNostrFromValue(result.Value, &parsedIdentifier{"d/testls", "_", true})
	if err != nil {
		t.Fatalf("extractNostrFromValue: %v", err)
	}
	if pk != testlsRootPubkey {
		t.Errorf("testls.bit (WSS) pubkey = %s, want %s", pk, testlsRootPubkey)
	}
}

func TestIntegration_ResolveAtTestlsBit_WSS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewElectrumClient()
	result, err := client.NameShowWithFallback(ctx, "d/testls", DefaultElectrumXServersWSS)
	if err != nil {
		t.Fatalf("NameShowWithFallback(d/testls, WSS): %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	pk, _, err := extractNostrFromValue(result.Value, &parsedIdentifier{"d/testls", "m", true})
	if err != nil {
		t.Fatalf("extractNostrFromValue: %v", err)
	}
	if pk != testlsMPubkey {
		t.Errorf("m@testls.bit (WSS) pubkey = %s, want %s", pk, testlsMPubkey)
	}
}

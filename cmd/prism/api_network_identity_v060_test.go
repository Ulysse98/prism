package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/transaction"
)

// Existing PoUW clients depend on status/health chainId retaining the
// genesis-only identity. Wallets and P2P need the configured ChainID instead.
func TestAPIExposesDistinctLegacyAndWalletNetworkIDs(t *testing.T) {
	for _, test := range []struct {
		name       string
		configured bool
	}{
		{name: "legacy", configured: false},
		{name: "configured", configured: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			chain, pos, wallets, err := createNode()
			if err != nil {
				t.Fatal(err)
			}
			if test.configured {
				chain.Config = blockchain.ChainConfig{
					ReservedAuthorities: reserved.AuthorityPolicy{
						Treasury: []string{wallets["Alice"].Address},
					},
				}
			}
			if !chain.ValidateChain(pos) {
				t.Fatal("configured test chain is invalid")
			}
			path := t.TempDir()
			if err := storage.Save(path, chain, pos, wallets); err != nil {
				t.Fatal(err)
			}
			api := &apiServer{dataPath: path}

			configuredID, err := chain.ChainID()
			if err != nil {
				t.Fatal(err)
			}
			legacyID := blockchain.MakeChainID(chain.Blocks[0].Hash)
			if test.configured && configuredID == legacyID {
				t.Fatal("fixture must distinguish configured and legacy ChainIDs")
			}
			if !test.configured && configuredID != legacyID {
				t.Fatal("default legacy network identity changed")
			}

			health := httptest.NewRecorder()
			api.handleHealth(health, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
			if health.Code != http.StatusOK {
				t.Fatalf("health HTTP %d: %s", health.Code, health.Body.String())
			}
			var healthData apiHealthResponse
			if err := json.Unmarshal(health.Body.Bytes(), &healthData); err != nil {
				t.Fatal(err)
			}
			if healthData.ChainID != legacyID || healthData.WalletChainID != configuredID {
				t.Fatalf("health network metadata mismatch: %+v", healthData)
			}

			status := httptest.NewRecorder()
			api.handleStatus(status, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
			if status.Code != http.StatusOK {
				t.Fatalf("status HTTP %d: %s", status.Code, status.Body.String())
			}
			var statusData apiStatusResponse
			if err := json.Unmarshal(status.Body.Bytes(), &statusData); err != nil {
				t.Fatal(err)
			}
			if statusData.ChainID != legacyID || statusData.WalletChainID != configuredID ||
				statusData.GenesisHash != chain.Blocks[0].Hash {
				t.Fatalf("status network metadata mismatch: %+v", statusData)
			}

			context := httptest.NewRecorder()
			api.handleWalletSigningContext(context, httptest.NewRequest(http.MethodGet, "/api/v1/transactions/context", nil))
			if context.Code != http.StatusOK {
				t.Fatalf("context HTTP %d: %s", context.Code, context.Body.String())
			}
			var contextData struct {
				ChainID            string `json:"chainId"`
				GenesisHash        string `json:"genesisHash"`
				TransactionVersion uint8  `json:"transactionVersion"`
			}
			if err := json.Unmarshal(context.Body.Bytes(), &contextData); err != nil {
				t.Fatal(err)
			}
			if contextData.ChainID != configuredID ||
				contextData.GenesisHash != chain.Blocks[0].Hash ||
				contextData.TransactionVersion != transaction.VersionV2 {
				t.Fatalf("signing context mismatch: %+v", contextData)
			}
		})
	}
}

// A signature for an alternative configured chain is invalid even when
// its genesis is identical. This is distinct from nonce replay prevention.
func TestWalletV2RejectsAnotherConfigurationWithSameGenesis(t *testing.T) {
	chain, pos, wallets, err := createNode()
	if err != nil {
		t.Fatal(err)
	}
	alice := wallets["Alice"]
	bob := wallets["Bob"]
	chain.Config = blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{alice.Address},
		},
	}
	alternateConfig := blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{bob.Address},
		},
	}
	ownChainID, err := chain.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	alternateChainID, err := blockchain.MakeConfiguredChainID(chain.Blocks[0].Hash, alternateConfig)
	if err != nil {
		t.Fatal(err)
	}
	if ownChainID == alternateChainID {
		t.Fatal("different configured networks unexpectedly share a ChainID")
	}

	path := t.TempDir()
	if err := storage.Save(path, chain, pos, wallets); err != nil {
		t.Fatal(err)
	}
	api := &apiServer{
		dataPath: path, computeMarket: compute.NewMarketplace(), walletTransfersEnabled: true,
	}

	wrong, err := transaction.NewV2(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex(), alternateChainID, chain.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := wrong.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	rejected := requestWalletTransfer(t, api, ownChainID, wrong)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("foreign configured network was accepted: HTTP %d: %s", rejected.Code, rejected.Body.String())
	}

	valid, err := transaction.NewV2(alice.Address, bob.Address, 1, 0, alice.PublicKeyHex(), ownChainID, chain.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := valid.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	accepted := requestWalletTransfer(t, api, ownChainID, valid)
	if accepted.Code != http.StatusCreated {
		t.Fatalf("valid configured network transfer rejected: HTTP %d: %s", accepted.Code, accepted.Body.String())
	}
}

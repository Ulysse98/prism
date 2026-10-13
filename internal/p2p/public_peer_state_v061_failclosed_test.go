package p2p

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/storage"
	"prism/internal/wallet"
)

type v061PeerModeFixture struct {
	dir     string
	chain   *blockchain.Blockchain
	pos     *consensus.ProofOfStake
	private map[string]*wallet.Wallet
	public  map[string]*wallet.Wallet
}

func makeV061PeerModeFixture(t *testing.T) v061PeerModeFixture {
	t.Helper()
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.NewBlockchain(map[string]uint64{
		alice.Address: 100,
		bob.Address:   50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.LockStake(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	pos := consensus.NewProofOfStake()
	if err := pos.Register(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	if !chain.ValidateChain(pos) {
		t.Fatal("invalid test blockchain")
	}
	return v061PeerModeFixture{
		dir:   t.TempDir(),
		chain: chain,
		pos:   pos,
		private: map[string]*wallet.Wallet{
			"Alice": alice,
			"Bob":   bob,
		},
		public: map[string]*wallet.Wallet{
			"Alice": {Address: alice.Address},
			"Bob":   {Address: bob.Address},
		},
	}
}

func (f v061PeerModeFixture) seedPublicMetadata(t *testing.T) {
	t.Helper()
	chainFile, err := json.Marshal(storage.ChainFile{
		Blockchain: f.chain,
		Validators: f.pos.Validators,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "chain.json"), chainFile, 0600); err != nil {
		t.Fatal(err)
	}
	records := []storage.PublicWalletRecord{
		{Name: "Alice", Address: f.public["Alice"].Address},
		{Name: "Bob", Address: f.public["Bob"].Address},
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, "api-wallets.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func assertNoPrivateWalletFileV061(t *testing.T, dir string) {
	t.Helper()
	_, err := os.Lstat(filepath.Join(dir, "wallets.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected private wallets.json or stat error: %v", err)
	}
}

func assertUnchangedChainV061(t *testing.T, dir string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(filepath.Join(dir, "chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("rejected persistence modified chain.json")
	}
}

func TestV061PublicModeCreatesStickyMarker(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	f.seedPublicMetadata(t)
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.public); err != nil {
		t.Fatal(err)
	}
	found, err := inspectPublicObserverMarkerV061(f.dir)
	if err != nil || !found {
		t.Fatalf("valid public observer marker missing: found=%v err=%v", found, err)
	}
	assertNoPrivateWalletFileV061(t, f.dir)
	if _, _, _, err := storage.LoadPublic(f.dir); err != nil {
		t.Fatalf("public observer cannot reload: %v", err)
	}
}

func TestV061PublicModeRejectsMissingMetadataWithoutDirectoryHeuristic(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	f.seedPublicMetadata(t)
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.public); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.dir, "api-wallets.json")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(f.dir, "chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, wallets := range map[string]map[string]*wallet.Wallet{
		"keyless":            f.public,
		"unexpected-private": f.private,
	} {
		t.Run(name, func(t *testing.T) {
			if err := persistPeerStateV061(f.dir, f.chain, f.pos, wallets); err == nil {
				t.Fatal("observer metadata loss must block persistence")
			}
			assertNoPrivateWalletFileV061(t, f.dir)
			assertUnchangedChainV061(t, f.dir, before)
		})
	}
}

func TestV061KeylessPeerNeverFallsBackWithoutMetadata(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.public); err == nil {
		t.Fatal("keyless peer silently fell back to private storage")
	}
	assertNoPrivateWalletFileV061(t, f.dir)
	if _, err := os.Stat(filepath.Join(f.dir, "chain.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected chain state write or stat error: %v", err)
	}
}

func TestV061ExistingChainWithoutWalletMetadataRefusesPrivateFallback(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	f.seedPublicMetadata(t)
	if err := os.Remove(filepath.Join(f.dir, "api-wallets.json")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(f.dir, "chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.private); err == nil {
		t.Fatal("existing chain must not silently recreate private wallets")
	}
	assertNoPrivateWalletFileV061(t, f.dir)
	assertUnchangedChainV061(t, f.dir, before)
}

func TestV061PublicModeRejectsPrivateKeysWithoutWrites(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	f.seedPublicMetadata(t)
	before, err := os.ReadFile(filepath.Join(f.dir, "chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.private); err == nil {
		t.Fatal("public observer accepted private key material")
	}
	assertNoPrivateWalletFileV061(t, f.dir)
	assertUnchangedChainV061(t, f.dir, before)
	if _, err := os.Stat(filepath.Join(f.dir, publicObserverMarkerV061)); !os.IsNotExist(err) {
		t.Fatalf("invalid public save unexpectedly wrote marker: %v", err)
	}
}

func TestV061PublicModeRejectsTamperedMarker(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	f.seedPublicMetadata(t)
	if err := os.WriteFile(filepath.Join(f.dir, publicObserverMarkerV061), []byte("tampered\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(f.dir, "chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.public); err == nil {
		t.Fatal("invalid marker should fail closed")
	}
	assertNoPrivateWalletFileV061(t, f.dir)
	assertUnchangedChainV061(t, f.dir, before)
}

func TestV061PrivateModeWorksWithExistingMetadata(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	if err := storage.Save(f.dir, f.chain, f.pos, f.private); err != nil {
		t.Fatal(err)
	}
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.private); err != nil {
		t.Fatalf("valid private node save refused: %v", err)
	}
	if _, _, _, err := storage.Load(f.dir); err != nil {
		t.Fatalf("private node cannot reload: %v", err)
	}
	if found, err := inspectPublicObserverMarkerV061(f.dir); err != nil || found {
		t.Fatalf("private peer acquired public marker: found=%v err=%v", found, err)
	}
}

func TestV061NewPrivateModeRequiresCompleteSigningKeys(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	partial := map[string]*wallet.Wallet{
		"Alice": {Address: f.private["Alice"].Address, PrivateKey: f.private["Alice"].PrivateKey[:12]},
	}
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, partial); err == nil {
		t.Fatal("partial key was incorrectly treated as private wallet")
	}
	assertNoPrivateWalletFileV061(t, f.dir)
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.private); err != nil {
		t.Fatalf("new private node initialization refused: %v", err)
	}
	if _, _, _, err := storage.Load(f.dir); err != nil {
		t.Fatalf("new private state cannot reload: %v", err)
	}
}

func TestV061PrivateModeRejectsPublicWalletMap(t *testing.T) {
	f := makeV061PeerModeFixture(t)
	if err := storage.Save(f.dir, f.chain, f.pos, f.private); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(f.dir, "chain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := persistPeerStateV061(f.dir, f.chain, f.pos, f.public); err == nil {
		t.Fatal("private node accepted keyless wallet map")
	}
	assertUnchangedChainV061(t, f.dir, before)
}

package blockchain

import (
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/escrow"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func escrowFundingFixtureV065(t *testing.T) (*Blockchain, *consensus.ProofOfStake, *wallet.Wallet, *wallet.Wallet) {
	t.Helper()
	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	bc, err := NewBlockchain(map[string]uint64{alice.Address: 100, bob.Address: 5})
	if err != nil {
		t.Fatal(err)
	}
	if err := bc.LockStake(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	pos := consensus.NewProofOfStake()
	if err := pos.Register(alice.Address, 10); err != nil {
		t.Fatal(err)
	}
	return bc, pos, alice, bob
}

func testEscrowAuthV065(t *testing.T, bc *Blockchain, requester *wallet.Wallet, height, nonce, amount uint64, jobID string) escrow.FundingAuthorization {
	t.Helper()
	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	f := escrow.Funding{ChainID: chainID, GenesisHash: bc.Blocks[0].Hash, JobID: jobID, Requester: requester.Address, Amount: amount, FundedAtHeight: height, ExpiresAtHeight: height + 20}
	a, err := escrow.SignFunding(f, nonce, requester.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestEscrowFundingV065LocksSpendableWithoutMinting(t *testing.T) {
	bc, pos, alice, bob := escrowFundingFixtureV065(t)
	originalSupply, err := bc.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	auth := testEscrowAuthV065(t, bc, alice, 1, 0, 40, strings.Repeat("a", 64))
	block, err := bc.AddEscrowFundingBlockV065(auth, alice.Address, pos)
	if err != nil {
		t.Fatal(err)
	}
	if block.Height != 1 || len(block.EscrowFundings) != 1 || !bc.ValidateChain(pos) {
		t.Fatal("funding block not valid")
	}
	locked, err := bc.GetEscrowLockedBalancesV065()
	if err != nil {
		t.Fatal(err)
	}
	if locked[alice.Address] != 40 {
		t.Fatalf("locked %d", locked[alice.Address])
	}
	gross, err := bc.BalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	available, err := bc.AvailableBalanceOf(alice.Address)
	if err != nil {
		t.Fatal(err)
	}
	if available != gross-50 {
		t.Fatalf("expected gross minus 40 escrow and 10 stake, gross=%d available=%d", gross, available)
	}
	supply, err := bc.TotalSupply()
	if err != nil {
		t.Fatal(err)
	}
	if supply != originalSupply+block.Reward {
		t.Fatalf("escrow minted/burned funds: before=%d after=%d reward=%d", originalSupply, supply, block.Reward)
	}
	chainID, err := bc.ChainID()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := transaction.NewV2(alice.Address, bob.Address, available+1, 0, alice.PublicKeyHex(), chainID, bc.Blocks[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Sign(alice.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if _, err := bc.AddBlock([]transaction.Transaction{tx}, nil, alice.Address, pos); err == nil {
		t.Fatal("overspending locked funds accepted")
	}
	if len(bc.Blocks) != 2 {
		t.Fatal("rejected spend mutated chain")
	}
}

func TestEscrowFundingV065RejectsTamperReplayAndOvercommit(t *testing.T) {
	bc, pos, alice, _ := escrowFundingFixtureV065(t)
	auth := testEscrowAuthV065(t, bc, alice, 1, 0, 40, strings.Repeat("b", 64))
	if _, err := bc.AddEscrowFundingBlockV065(auth, alice.Address, pos); err != nil {
		t.Fatal(err)
	}
	next := testEscrowAuthV065(t, bc, alice, 2, 1, 40, strings.Repeat("b", 64))
	if _, err := bc.AddEscrowFundingBlockV065(next, alice.Address, pos); err == nil {
		t.Fatal("duplicate job accepted")
	}
	if len(bc.Blocks) != 2 {
		t.Fatal("replay mutated chain")
	}
	sameNonce := testEscrowAuthV065(t, bc, alice, 2, 0, 1, strings.Repeat("c", 64))
	if _, err := bc.AddEscrowFundingBlockV065(sameNonce, alice.Address, pos); err == nil {
		t.Fatal("duplicate nonce accepted")
	}
	over := testEscrowAuthV065(t, bc, alice, 2, 2, 100, strings.Repeat("d", 64))
	if _, err := bc.AddEscrowFundingBlockV065(over, alice.Address, pos); err == nil {
		t.Fatal("overcommit accepted")
	}
	tamper := testEscrowAuthV065(t, bc, alice, 2, 2, 1, strings.Repeat("e", 64))
	tamper.Funding.Amount++
	if _, err := bc.AddEscrowFundingBlockV065(tamper, alice.Address, pos); err == nil {
		t.Fatal("tampered signature accepted")
	}
	if len(bc.Blocks) != 2 {
		t.Fatal("invalid funding mutated chain")
	}
}

func TestEscrowFundingV065OldBlocksKeepHashes(t *testing.T) {
	bc, _, _, _ := escrowFundingFixtureV065(t)
	if CalculateHash(bc.Blocks[0]) != bc.Blocks[0].Hash {
		t.Fatal("legacy genesis hash changed")
	}
}

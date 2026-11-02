package finality

import (
	"bytes"
	"crypto/ed25519"
	"reflect"
	"strings"
	"testing"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

// These tests deliberately use only deterministic throwaway keys.
// They MUST NOT be interpreted as an active BFT consensus qualification.
func v068Fixture(t *testing.T) ([]consensus.Validator, []ed25519.PrivateKey, string) {
	t.Helper()
	stakes := []uint64{40, 26, 34}
	validators := make([]consensus.Validator, 0, len(stakes))
	keys := make([]ed25519.PrivateKey, 0, len(stakes))
	for i, stake := range stakes {
		seed := bytes.Repeat([]byte{byte(i + 1)}, ed25519.SeedSize)
		key := ed25519.NewKeyFromSeed(seed)
		keys = append(keys, key)
		validators = append(validators, consensus.Validator{
			Address: wallet.AddressFromPublicKey(key.Public().(ed25519.PublicKey)),
			Stake:   stake,
		})
	}
	hash, err := ValidatorSetHash(validators)
	if err != nil {
		t.Fatal(err)
	}
	return validators, keys, hash
}

func v068Context(setHash, block string, round uint64, step string) VoteContext {
	return VoteContext{
		ChainID:          "prism-v068-isolated-lab",
		GenesisHash:      strings.Repeat("ab", 32),
		Height:           8,
		Round:            round,
		Step:             step,
		BlockHash:        block,
		ValidatorSetHash: setHash,
	}
}

func v068Certificate(t *testing.T, ctx VoteContext, keys []ed25519.PrivateKey, indices ...int) Certificate {
	t.Helper()
	cert := Certificate{Context: ctx}
	for _, i := range indices {
		vote, err := SignVote(ctx, keys[i])
		if err != nil {
			t.Fatal(err)
		}
		cert.Votes = append(cert.Votes, vote)
	}
	return cert
}

func TestV068SafetyBaselineQuorumRejectsUnderweightDuplicateTamperAndSnapshot(t *testing.T) {
	validators, keys, setHash := v068Fixture(t)
	ctx := v068Context(setHash, strings.Repeat("cd", 32), 0, stepPrevote)

	// 40 + 26 = 66/100: below the strict >2/3 threshold.
	underweight := v068Certificate(t, ctx, keys, 0, 1)
	if _, _, err := VerifyQuorum(validators, underweight); err == nil {
		t.Fatal("underweight quorum accepted")
	}

	// 40 + 34 = 74/100: sufficient stake.
	valid := v068Certificate(t, ctx, keys, 0, 2)
	if signed, total, err := VerifyQuorum(validators, valid); err != nil || signed != 74 || total != 100 {
		t.Fatalf("valid quorum rejected: signed=%d total=%d err=%v", signed, total, err)
	}

	duplicate := v068Certificate(t, ctx, keys, 0, 2, 0)
	if _, _, err := VerifyQuorum(validators, duplicate); err == nil {
		t.Fatal("duplicate voter accepted")
	}

	tampered := v068Certificate(t, ctx, keys, 0, 2)
	tampered.Votes[0].Signature = strings.Repeat("00", ed25519.SignatureSize)
	if _, _, err := VerifyQuorum(validators, tampered); err == nil {
		t.Fatal("tampered signature accepted")
	}

	changed := append([]consensus.Validator(nil), validators...)
	changed[0].Stake++
	if _, _, err := VerifyQuorum(changed, valid); err == nil {
		t.Fatal("stale validator-set snapshot accepted")
	}
}

func TestV068SafetyBaselineConflictingQuorumsAreNotFinality(t *testing.T) {
	validators, keys, setHash := v068Fixture(t)
	a := v068Certificate(t, v068Context(setHash, strings.Repeat("cd", 32), 0, stepPrecommit), keys, 0, 2)
	b := v068Certificate(t, v068Context(setHash, strings.Repeat("ef", 32), 0, stepPrecommit), keys, 0, 2)

	// Intentionally demonstrate missing *global signer isolation*: both quorums
	// verify with the same unguarded test keys. Never call them finality proofs.
	if _, _, err := VerifyQuorum(validators, a); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyQuorum(validators, b); err != nil {
		t.Fatal(err)
	}
}

func TestV068SafetyBaselineCrossNetworkPoLCDoesNotReleaseLock(t *testing.T) {
	validators, keys, setHash := v068Fixture(t)
	genesis := strings.Repeat("ab", 32)
	blockA := strings.Repeat("cd", 32)
	blockB := strings.Repeat("ef", 32)

	machine, err := NewBFTLockMachineV065("prism-v068-isolated-lab", genesis, 8, validators)
	if err != nil {
		t.Fatal(err)
	}
	if choice, err := machine.ChoosePrevoteV065(blockA, true, nil); err != nil || choice != blockA {
		t.Fatalf("initial prevote failed: choice=%q err=%v", choice, err)
	}
	prevotes := v068Certificate(t, v068Context(setHash, blockA, 0, stepPrevote), keys, 0, 2)
	if choice, err := machine.EnterPrecommitV065(&prevotes, true); err != nil || choice != blockA {
		t.Fatalf("lock transition failed: choice=%q err=%v", choice, err)
	}
	if err := machine.AdvanceRoundV065(2); err != nil {
		t.Fatal(err)
	}
	before := machine.SnapshotV065()
	if !before.HasLock || before.LockedBlock != blockA {
		t.Fatal("lock lost before adversarial test")
	}

	wrongNetworkCtx := v068Context(setHash, blockB, 1, stepPrevote)
	wrongNetworkCtx.ChainID = "attacker-chain"
	forgedJustification := v068Certificate(t, wrongNetworkCtx, keys, 0, 2)
	if _, err := machine.ChoosePrevoteV065(blockB, true, &forgedJustification); err == nil {
		t.Fatal("wrong-network unlock proof accepted")
	}
	if !reflect.DeepEqual(machine.SnapshotV065(), before) {
		t.Fatal("rejected PoLC mutated lock snapshot")
	}

	// No acceptable PoLC: validator must continue prevoting the locked block.
	choice, err := machine.ChoosePrevoteV065(blockB, true, nil)
	if err != nil || choice != blockA {
		t.Fatalf("lock not respected without unlock proof: choice=%q err=%v", choice, err)
	}
}

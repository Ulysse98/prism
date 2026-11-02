package finality

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// Dedicated, deterministic throwaway operator keys. They are NEVER validators'
// production keys, and this fixture does not establish a real trust root.
func v068SignedCheckpointFixture(t *testing.T) (
	*blockchain.Blockchain, []consensus.Validator,
	SignedValidatorCheckpointV068, CheckpointTrustPolicyV068, ed25519.PrivateKey,
) {
	t.Helper()
	bc, validators, pin := v068PinnedSnapshotFixture(t)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x68}, ed25519.SeedSize))
	policy := CheckpointTrustPolicyV068{
		ChainID: pin.ChainID, GenesisHash: pin.GenesisHash,
		MinParentHeight: 0, MaxParentHeight: 100,
		Authorities: []CheckpointAuthorityV068{{
			ID: "lab-root-a", PublicKey: key.Public().(ed25519.PublicKey),
			FirstParentHeight: 0, LastParentHeight: 100,
		}},
	}
	signed := SignedValidatorCheckpointV068{Anchor: pin, AuthorityID: "lab-root-a"}
	v068ResignCheckpoint(t, &signed, key)
	return bc, validators, signed, policy, key
}

func v068ResignCheckpoint(t *testing.T, s *SignedValidatorCheckpointV068, key ed25519.PrivateKey) {
	t.Helper()
	payload, err := signedCheckpointPayloadV068(*s)
	if err != nil {
		t.Fatal(err)
	}
	s.SignatureHex = hex.EncodeToString(ed25519.Sign(key, payload))
}

func TestV068SignedCheckpointAcceptsTrustedPolicyAndExactSnapshot(t *testing.T) {
	bc, validators, signed, policy, _ := v068SignedCheckpointFixture(t)
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, signed, policy); err != nil {
		t.Fatal(err)
	}
	reordered := append([]consensus.Validator(nil), validators...)
	reordered[0], reordered[2] = reordered[2], reordered[0]
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, reordered, signed, policy); err != nil {
		t.Fatalf("reordered canonical validators rejected: %v", err)
	}
}

func TestV068SignedCheckpointRejectsTamperingWrongNetworkAndUnknownSigner(t *testing.T) {
	bc, validators, signed, policy, key := v068SignedCheckpointFixture(t)
	variants := []struct {
		name   string
		mutate func(*SignedValidatorCheckpointV068)
	}{
		{"signature", func(s *SignedValidatorCheckpointV068) { s.SignatureHex = strings.Repeat("00", ed25519.SignatureSize) }},
		{"uppercase_signature", func(s *SignedValidatorCheckpointV068) { s.SignatureHex = strings.ToUpper(s.SignatureHex) }},
		{"unknown_authority", func(s *SignedValidatorCheckpointV068) { s.AuthorityID = "attacker" }},
		{"wrong_chain", func(s *SignedValidatorCheckpointV068) { s.Anchor.ChainID = "prism-other-network" }},
		{"wrong_genesis", func(s *SignedValidatorCheckpointV068) { s.Anchor.GenesisHash = strings.Repeat("cc", 32) }},
		{"wrong_parent", func(s *SignedValidatorCheckpointV068) { s.Anchor.ParentHash = strings.Repeat("cc", 32) }},
		{"wrong_set", func(s *SignedValidatorCheckpointV068) { s.Anchor.ValidatorSetHash = strings.Repeat("cc", 32) }},
		{"wrong_height", func(s *SignedValidatorCheckpointV068) { s.Anchor.ParentHeight++ }},
	}
	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			x := signed
			tc.mutate(&x)
			if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, x, policy); err == nil {
				t.Fatal("tampered or untrusted checkpoint accepted")
			}
		})
	}
	// Even a fresh, valid operator signature cannot make an incorrect
	// checkpoint match the already pinned parent state and locked stakes.
	selfConsistentForgery := signed
	selfConsistentForgery.Anchor.ValidatorSetHash = strings.Repeat("dd", 32)
	v068ResignCheckpoint(t, &selfConsistentForgery, key)
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, selfConsistentForgery, policy); err == nil {
		t.Fatal("operator-signed but parent-incompatible pin accepted")
	}
	other := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x69}, ed25519.SeedSize))
	forgedSigner := signed
	v068ResignCheckpoint(t, &forgedSigner, other)
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, forgedSigner, policy); err == nil {
		t.Fatal("attacker signature accepted as trusted operator")
	}
}

func TestV068SignedCheckpointRejectsRevokedStaleOrInvalidPolicy(t *testing.T) {
	bc, validators, signed, policy, _ := v068SignedCheckpointFixture(t)
	tests := []struct {
		name   string
		mutate func(*CheckpointTrustPolicyV068)
	}{
		{"revoked", func(p *CheckpointTrustPolicyV068) { p.Authorities[0].Revoked = true }},
		{"stale_below_floor", func(p *CheckpointTrustPolicyV068) { p.MinParentHeight = 1 }},
		{"signer_not_yet_active", func(p *CheckpointTrustPolicyV068) { p.Authorities[0].FirstParentHeight = 1 }},
		{"bad_network", func(p *CheckpointTrustPolicyV068) { p.ChainID = "prism-untrusted" }},
		{"bad_genesis", func(p *CheckpointTrustPolicyV068) { p.GenesisHash = strings.Repeat("ab", 32) }},
		{"no_authorities", func(p *CheckpointTrustPolicyV068) { p.Authorities = nil }},
		{"invalid_height_window", func(p *CheckpointTrustPolicyV068) { p.MinParentHeight = 101; p.MaxParentHeight = 100 }},
		{"invalid_key", func(p *CheckpointTrustPolicyV068) { p.Authorities[0].PublicKey = []byte{1} }},
		{"invalid_id", func(p *CheckpointTrustPolicyV068) { p.Authorities[0].ID = "operator with spaces" }},
		{"invalid_authority_window", func(p *CheckpointTrustPolicyV068) {
			p.Authorities[0].FirstParentHeight = 101
			p.Authorities[0].LastParentHeight = 100
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := policy
			p.Authorities = append([]CheckpointAuthorityV068(nil), policy.Authorities...)
			tc.mutate(&p)
			if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, signed, p); err == nil {
				t.Fatal("invalid or untrusted policy accepted")
			}
		})
	}
	p := policy
	p.Authorities = append(append([]CheckpointAuthorityV068(nil), policy.Authorities...), policy.Authorities[0])
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, signed, p); err == nil {
		t.Fatal("duplicated authority ID accepted")
	}
	p = policy
	p.Authorities = append(append([]CheckpointAuthorityV068(nil), policy.Authorities...), policy.Authorities[0])
	p.Authorities[1].ID = "distinct-alias-same-key"
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, signed, p); err == nil {
		t.Fatal("aliased duplicate public key accepted")
	}
}

func TestV068SignedCheckpointRejectsAuthorityHeightExpiry(t *testing.T) {
	bc, validators, signed, policy, key := v068SignedCheckpointFixture(t)
	signed.Anchor.ParentHeight = 5
	v068ResignCheckpoint(t, &signed, key)
	policy.Authorities[0].LastParentHeight = 4
	if err := VerifySignedPinnedValidatorSnapshotV068(bc, validators, signed, policy); err == nil {
		t.Fatal("checkpoint after authority validity interval accepted")
	}
}

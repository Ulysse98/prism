//go:build linux

package governancev069

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

func newRotationKey() ed25519.PrivateKey {
	s := sha256.Sum256([]byte("PRISM v0.69 1B8 ROTATION TEST KEY ONLY"))
	return ed25519.NewKeyFromSeed(s[:])
}

func TestDualSignatureRotationCandidate(t *testing.T) {
	// This is a pure offline fixture, not a runtime authority update.
	c := AdvisoryBallotClosure{
		Version: AdvisoryBallotClosureVersion, Network: "devnet", ChainID: "prism-testnet", GenesisSHA256: strings.Repeat("a", 64),
		ProposalSHA256: strings.Repeat("b", 64), SnapshotSHA256: strings.Repeat("c", 64), BallotSetSHA256: strings.Repeat("d", 64),
		TallyReceiptSHA256: strings.Repeat("e", 64), ReservationAuditSHA256: strings.Repeat("f", 64), ClosedAtHeight: 111,
	}
	digest := localBallotClosureCommitment(c)
	c.ClosureSHA256 = hex.EncodeToString(digest[:])
	a, err := SignAdvisoryClosureAuthorization(c, 3, closureOperatorKey())
	if err != nil {
		t.Fatal(err)
	}
	c.Authorization = a
	f, err := BuildAdvisoryClosureRecoveryFloor(c, closureOperatorPin(), 7)
	if err != nil {
		t.Fatal(err)
	}
	r, err := SignAdvisoryAuthorityRotationCandidate(f, newRotationKey(), closureOperatorKey(), 11)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyAdvisoryAuthorityRotationCandidate(r, f); err != nil {
		t.Fatal(err)
	}
	if r.ToEpoch != 8 || r.FromEpoch != 7 {
		t.Fatal("wrong epoch progression")
	}
	const goldenRotation = "a302931fe84910ec4d3beeb8d37f02ef5c0909fb968253e8a4f4f01d5ff8c74b"
	actualDigest := authorityRotationDigest(r)
	if hex.EncodeToString(actualDigest[:]) != goldenRotation {
		t.Fatalf("canonical rotation digest changed: got %x", actualDigest)
	}
	attacks := []struct {
		name   string
		change func(*AdvisoryAuthorityRotation)
	}{
		{"different chain", func(r *AdvisoryAuthorityRotation) { r.ChainID = "foreign" }},
		{"different genesis", func(r *AdvisoryAuthorityRotation) { r.GenesisSHA256 = strings.Repeat("0", 64) }},
		{"different proposal", func(r *AdvisoryAuthorityRotation) { r.ProposalSHA256 = strings.Repeat("0", 64) }},
		{"different closure", func(r *AdvisoryAuthorityRotation) { r.ClosureSHA256 = strings.Repeat("0", 64) }},
		{"old key swap", func(r *AdvisoryAuthorityRotation) { r.FromAuthorityHex = r.ToAuthorityHex }},
		{"new key swap", func(r *AdvisoryAuthorityRotation) { r.ToAuthorityHex = r.FromAuthorityHex }},
		{"new key signature", func(r *AdvisoryAuthorityRotation) { r.NewSignatureHex = strings.Repeat("0", 128) }},
		{"old key signature", func(r *AdvisoryAuthorityRotation) { r.OldSignatureHex = strings.Repeat("0", 128) }},
		{"nonce", func(r *AdvisoryAuthorityRotation) { r.Nonce++ }},
		{"skip epoch", func(r *AdvisoryAuthorityRotation) { r.ToEpoch += 2 }},
		{"rollback epoch", func(r *AdvisoryAuthorityRotation) { r.FromEpoch-- }},
		{"zero epoch", func(r *AdvisoryAuthorityRotation) { r.FromEpoch = 0 }},
	}
	for _, a := range attacks {
		t.Run(a.name, func(t *testing.T) {
			x := r
			a.change(&x)
			if err := VerifyAdvisoryAuthorityRotationCandidate(x, f); err == nil {
				t.Fatal("tampered rotation candidate accepted")
			}
		})
	}
	other := f
	other.AuthorityEpoch++
	d := closureFloorCommitment(other)
	other.FloorSHA256 = hex.EncodeToString(d[:])
	if err := VerifyAdvisoryAuthorityRotationCandidate(r, other); err == nil {
		t.Fatal("old rotation replayed against new epoch")
	}
	other = f
	other.AuthorityPublicKeyHex = r.ToAuthorityHex
	d = closureFloorCommitment(other)
	other.FloorSHA256 = hex.EncodeToString(d[:])
	if err := VerifyAdvisoryAuthorityRotationCandidate(r, other); err == nil {
		t.Fatal("wrong independent old authority accepted")
	}
	if _, err := SignAdvisoryAuthorityRotationCandidate(f, newRotationKey(), newRotationKey(), 12); err == nil {
		t.Fatal("untrusted old key accepted")
	}
	if _, err := SignAdvisoryAuthorityRotationCandidate(f, closureOperatorKey(), closureOperatorKey(), 13); err == nil {
		t.Fatal("same-key rotation accepted")
	}
	f.AuthorityEpoch = math.MaxUint64
	d = closureFloorCommitment(f)
	f.FloorSHA256 = hex.EncodeToString(d[:])
	if _, err := SignAdvisoryAuthorityRotationCandidate(f, newRotationKey(), closureOperatorKey(), 14); err == nil {
		t.Fatal("epoch overflow accepted")
	}
}

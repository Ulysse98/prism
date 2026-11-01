package crosschain

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func signedOperatorTrustFixtureV067(t *testing.T, f anchorIncludedFixtureV067, cp PinnedSourceCheckpointV067) (ed25519.PublicKey, SignedOperatorTrustV067) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	return pub, SignedOperatorTrustV067{
		Policy: f.candidate.policy, Checkpoint: cp,
		Signature: ed25519.Sign(priv, message),
	}
}

func TestLocalOperatorAdmissionV067RealCryptoAndRestartReplay(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	pub, signed := signedOperatorTrustFixtureV067(t, f, cp)
	root := t.TempDir()
	store, err := NewLocalReplayStoreV067(root)
	if err != nil {
		t.Fatal(err)
	}
	operator, err := newLocalOperatorAdmitterV067(store, pub, signed, f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	candidate := realAdmissionCandidateV067(f)
	// Mutating the source bundle after initialization cannot change the pinned values.
	signed.Policy.ApprovedVerifier = "untrusted-verifier"
	signed.Checkpoint.CheckpointBlockHash = strings.Repeat("f", 64)
	if _, err := operator.Admit(candidate); err != nil {
		t.Fatalf("valid independently signed proof rejected: %v", err)
	}
	if _, err := operator.Admit(candidate); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("same-process replay not rejected: %v", err)
	}
	reopened, err := NewLocalReplayStoreV067(root)
	if err != nil {
		t.Fatal(err)
	}
	// The trust bundle itself was deliberately mutated above; a fresh signed
	// bundle is required for any new operator context.
	freshPub, freshSigned := signedOperatorTrustFixtureV067(t, f, cp)
	newOperator, err := newLocalOperatorAdmitterV067(reopened, freshPub, freshSigned, f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newOperator.Admit(candidate); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("reopened-store replay not rejected: %v", err)
	}
}

func TestLocalOperatorAdmissionV067FailClosedTrustAndCandidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ed25519.PublicKey, *SignedOperatorTrustV067)
	}{
		{"tampered-policy", func(_ *ed25519.PublicKey, s *SignedOperatorTrustV067) {
			s.Policy.ApprovedVerifier = "untrusted-verifier"
		}},
		{"tampered-checkpoint", func(_ *ed25519.PublicKey, s *SignedOperatorTrustV067) {
			s.Checkpoint.CheckpointBlockHash = strings.Repeat("f", 64)
		}},
		{"bad-signature", func(_ *ed25519.PublicKey, s *SignedOperatorTrustV067) { s.Signature[0] ^= 0x01 }},
		{"wrong-operator-root", func(k *ed25519.PublicKey, _ *SignedOperatorTrustV067) { (*k)[0] ^= 0x01 }},
		{"missing-operator-root", func(k *ed25519.PublicKey, _ *SignedOperatorTrustV067) { *k = nil }},
		{"zero-checkpoint-depth", func(_ *ed25519.PublicKey, s *SignedOperatorTrustV067) { s.Checkpoint.MinSuccessorBlocks = 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, cp := newCheckpointFixtureV067(t)
			pub, signed := signedOperatorTrustFixtureV067(t, f, cp)
			store, err := NewLocalReplayStoreV067(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&pub, &signed)
			if _, err := newLocalOperatorAdmitterV067(store, pub, signed, f.chain, f.pos); !errors.Is(err, ErrOperatorTrustV067) {
				t.Fatalf("untrusted or tampered operator context accepted: %v", err)
			}
			entries, err := os.ReadDir(store.dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("rejected trust must not reserve slot: %v, %v", entries, err)
			}
		})
	}
}

func TestLocalOperatorAdmissionV067CandidateCannotReplacePin(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	pub, signed := signedOperatorTrustFixtureV067(t, f, cp)
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	operator, err := newLocalOperatorAdmitterV067(store, pub, signed, f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	candidate := realAdmissionCandidateV067(f)
	invalid := candidate
	invalid.Intent.DestinationChainID = "eip155:10143"
	invalid.Intent.DestinationRegistry = "0x2222222222222222222222222222222222222222"
	if _, err := operator.Admit(invalid); err == nil {
		t.Fatal("candidate-controlled destination bypassed operator pin")
	}
	invalid = candidate
	invalid.Proof.Signature = "00"
	if _, err := operator.Admit(invalid); err == nil {
		t.Fatal("tampered PoUW signature admitted")
	}
	if _, err := operator.Admit(candidate); err != nil {
		t.Fatalf("invalid candidates poisoned valid proof slot: %v", err)
	}
}

// Source-level API regression: external packages must not be able to invoke the
// old exported admission function with caller-selected trusted parameters.
func TestLocalOperatorAdmissionV067NoPublicRawAdmission(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "local_anchor_admission_v067.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		switch fn.Name.Name {
		case "AdmitLocalAnchorCandidateV067":
			t.Fatal("unsafe public caller-pinned admission API must remain private")
		case "admitLocalAnchorCandidateV067":
			found++
		}
	}
	if found != 1 {
		t.Fatalf("expected one private verified admission function, got %d", found)
	}
}

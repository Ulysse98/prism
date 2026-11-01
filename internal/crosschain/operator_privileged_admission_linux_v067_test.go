//go:build linux

package crosschain

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivilegedAdmissionV067SignedPinnedSynchronizedAndReplay(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	source, err := NewLocalSynchronizedOperatorSourceV067(f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	path, pin, _, private := pinnedRootFixtureV067(t)
	message, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedOperatorTrustV067{
		Policy: f.candidate.policy, Checkpoint: cp,
		Signature: ed25519.Sign(private, message),
	}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	op, err := NewLocalOperatorAdmitterFromPinnedSynchronizedSourceV067(store, path, pin, signed, source)
	if err != nil {
		t.Fatal(err)
	}
	candidate := realAdmissionCandidateV067(f)
	if _, err := op.Admit(candidate); err != nil {
		t.Fatalf("valid pinned/synchronized proof denied: %v", err)
	}
	if _, err := op.Admit(candidate); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("local replay unexpectedly accepted: %v", err)
	}
}

func TestPrivilegedAdmissionV067FailsClosedOnUntrustedInputs(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	source, err := NewLocalSynchronizedOperatorSourceV067(f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	path, pin, _, private := pinnedRootFixtureV067(t)
	message, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedOperatorTrustV067{Policy: f.candidate.policy, Checkpoint: cp, Signature: ed25519.Sign(private, message)}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		name   string
		path   string
		pin    string
		source *LocalSynchronizedOperatorSourceV067
		store  *LocalReplayStoreV067
		signed SignedOperatorTrustV067
	}
	cases := []testCase{
		{"nil-source", path, pin, nil, store, signed},
		{"nil-store", path, pin, source, nil, signed},
		{"missing-root", filepath.Join(t.TempDir(), "absent"), pin, source, store, signed},
		{"bad-fingerprint", path, strings.Repeat("0", 64), source, store, signed},
	}
	_, otherPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongSignature := signed
	wrongSignature.Signature = ed25519.Sign(otherPrivate, message)
	cases = append(cases, testCase{"wrong-operator-signature", path, pin, source, store, wrongSignature})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewLocalOperatorAdmitterFromPinnedSynchronizedSourceV067(
				tc.store, tc.path, tc.pin, tc.signed, tc.source,
			); !errors.Is(err, ErrOperatorTrustV067) {
				t.Fatalf("unsafe privileged constructor input accepted: %v", err)
			}
		})
	}
}

// This catches accidental re-export of a raw-key or raw-source admission path.
// It cannot constrain malicious code within the same Go package, or demonstrate
// that operator configuration truly came from a privileged independent channel.
func TestPrivilegedAdmissionV067OnlyOneExportedConstructor(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := "NewLocalOperatorAdmitterFromPinnedSynchronizedSourceV067"
	count := 0
	for _, entry := range files {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "NewLocalOperatorAdmitter") || !fn.Name.IsExported() {
				continue
			}
			if fn.Name.Name != allowed {
				t.Fatalf("unexpected exported privileged constructor: %s (%s)", fn.Name.Name, entry.Name())
			}
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one exported privileged constructor, got %d", count)
	}
}

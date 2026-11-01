//go:build linux

package crosschain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pinnedRootFixtureV067(t *testing.T) (string, string, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "operator-root.pub")
	if err := os.WriteFile(path, []byte(hex.EncodeToString(pub)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fp := sha256.Sum256(pub)
	return path, hex.EncodeToString(fp[:]), pub, priv
}

func TestPinnedOperatorRootV067ValidAndRotatedKeyRejected(t *testing.T) {
	path, pin, pub, _ := pinnedRootFixtureV067(t)
	loaded, err := LoadPinnedOperatorRootV067(path, pin)
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded) != string(pub) {
		t.Fatal("wrong pinned key loaded")
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(other)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPinnedOperatorRootV067(path, pin); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("unexpected acceptance of unpinned rotated key: %v", err)
	}
}

func TestPinnedOperatorRootV067RejectsUnsafeInputs(t *testing.T) {
	path, pin, _, _ := pinnedRootFixtureV067(t)
	cases := []struct {
		name  string
		setup func(*testing.T) (string, string)
	}{
		{"missing-pin", func(t *testing.T) (string, string) { return path, "" }},
		{"wrong-pin", func(t *testing.T) (string, string) { return path, strings.Repeat("0", 64) }},
		{"uppercase-pin", func(t *testing.T) (string, string) { return path, strings.ToUpper(strings.Repeat("a", 64)) }},
		{"relative-path", func(t *testing.T) (string, string) { return "operator-root.pub", pin }},
		{"world-readable", func(t *testing.T) (string, string) {
			p := filepath.Join(t.TempDir(), "shared.pub")
			b, _ := os.ReadFile(path)
			if err := os.WriteFile(p, b, 0644); err != nil {
				t.Fatal(err)
			}
			// os.WriteFile applies umask: force the unsafe mode being tested.
			if err := os.Chmod(p, 0644); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if mode := info.Mode().Perm(); mode != 0644 {
				t.Fatalf("unsafe root fixture mode %04o; want 0644", mode)
			}
			return p, pin
		}},
		{"symlink-file", func(t *testing.T) (string, string) {
			p := filepath.Join(t.TempDir(), "link.pub")
			if err := os.Symlink(path, p); err != nil {
				t.Fatal(err)
			}
			return p, pin
		}},
		{"symlink-parent", func(t *testing.T) (string, string) {
			p := filepath.Join(t.TempDir(), "linked-dir")
			if err := os.Symlink(filepath.Dir(path), p); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(p, filepath.Base(path)), pin
		}},
		{"directory", func(t *testing.T) (string, string) { return t.TempDir(), pin }},
		{"trailing-garbage", func(t *testing.T) (string, string) {
			p := filepath.Join(t.TempDir(), "bad.pub")
			b, _ := os.ReadFile(path)
			if err := os.WriteFile(p, append(b, 'x'), 0600); err != nil {
				t.Fatal(err)
			}
			return p, pin
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, fp := tc.setup(t)
			if _, err := LoadPinnedOperatorRootV067(p, fp); !errors.Is(err, ErrOperatorTrustV067) {
				t.Fatalf("unsafe root accepted: %v", err)
			}
		})
	}
}

func TestPinnedOperatorRootV067RealAdmissionAndReplay(t *testing.T) {
	f, cp := newCheckpointFixtureV067(t)
	path, pin, _, priv := pinnedRootFixtureV067(t)
	message, err := operatorTrustMessageV067(f.candidate.policy, cp)
	if err != nil {
		t.Fatal(err)
	}
	signed := SignedOperatorTrustV067{Policy: f.candidate.policy, Checkpoint: cp, Signature: ed25519.Sign(priv, message)}
	store, err := NewLocalReplayStoreV067(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	operator, err := NewLocalOperatorAdmitterFromPinnedRootV067(store, path, pin, signed, f.chain, f.pos)
	if err != nil {
		t.Fatal(err)
	}
	candidate := realAdmissionCandidateV067(f)
	if _, err := operator.Admit(candidate); err != nil {
		t.Fatalf("pinned root rejected valid proof: %v", err)
	}
	if _, err := operator.Admit(candidate); !errors.Is(err, ErrLocalReplayReservedV067) {
		t.Fatalf("expected replay: %v", err)
	}
	// New signature from a different operator cannot be validated using the
	// independently pinned original key.
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signed.Signature = ed25519.Sign(otherPriv, message)
	if _, err := NewLocalOperatorAdmitterFromPinnedRootV067(store, path, pin, signed, f.chain, f.pos); !errors.Is(err, ErrOperatorTrustV067) {
		t.Fatalf("unpinned operator signature accepted: %v", err)
	}
}

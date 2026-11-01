//go:build linux

package crosschain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// LoadPinnedOperatorRootV067 loads a public Ed25519 operator root from a
// local file and compares its SHA-256 against a separately pinned digest.
// The trusted caller MUST supply both path and expected digest from privileged
// configuration. NEVER accept either value from a candidate or API request.
// File ownership/mode and symlink checks are Linux-local defenses, not a
// substitute for controlling parent directories, users, or the pinned digest.
func LoadPinnedOperatorRootV067(filePath, pinnedSHA256 string) (ed25519.PublicKey, error) {
	if filePath == "" || filePath != filepath.Clean(filePath) || !filepath.IsAbs(filePath) {
		return nil, fmt.Errorf("%w: operator root path must be absolute and canonical", ErrOperatorTrustV067)
	}
	if len(pinnedSHA256) != 64 || strings.ToLower(pinnedSHA256) != pinnedSHA256 {
		return nil, fmt.Errorf("%w: invalid root pin format", ErrOperatorTrustV067)
	}
	expected, err := hex.DecodeString(pinnedSHA256)
	if err != nil || len(expected) != sha256.Size {
		return nil, fmt.Errorf("%w: malformed root pin", ErrOperatorTrustV067)
	}

	// Refuse symlinks in every existing path component, including parents.
	// The caller must also prevent concurrent namespace changes by untrusted
	// processes. This loop is NOT an atomic directory-walk/ownership guarantee.
	part := string(os.PathSeparator)
	for _, name := range strings.Split(strings.TrimPrefix(filePath, part), part) {
		if name == "" || name == "." || name == ".." {
			return nil, fmt.Errorf("%w: invalid root path component", ErrOperatorTrustV067)
		}
		part = filepath.Join(part, name)
		info, err := os.Lstat(part)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: unsafe root path component: %v", ErrOperatorTrustV067, err)
		}
	}
	before, err := os.Lstat(filePath)
	if err != nil || !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("%w: missing, nonregular or shared root file", ErrOperatorTrustV067)
	}
	if st, ok := before.Sys().(*syscall.Stat_t); !ok || st.Uid != uint32(os.Geteuid()) {
		return nil, fmt.Errorf("%w: root file must be owned by the operator process user", ErrOperatorTrustV067)
	}
	fd, err := syscall.Open(filePath, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: open root without symlink: %v", ErrOperatorTrustV067, err)
	}
	file := os.NewFile(uintptr(fd), filePath)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("%w: invalid root file handle", ErrOperatorTrustV067)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || after.Mode().Perm()&0077 != 0 || !os.SameFile(before, after) {
		return nil, fmt.Errorf("%w: root file changed or became unsafe", ErrOperatorTrustV067)
	}
	data, err := io.ReadAll(io.LimitReader(file, 67))
	if err != nil {
		return nil, fmt.Errorf("%w: read root: %v", ErrOperatorTrustV067, err)
	}
	text := string(data)
	if len(text) == 65 && text[64] == '\n' {
		text = text[:64]
	}
	if len(text) != 64 || strings.ToLower(text) != text {
		return nil, fmt.Errorf("%w: root key must be 64 lowercase hex characters", ErrOperatorTrustV067)
	}
	key, err := hex.DecodeString(text)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: invalid public key", ErrOperatorTrustV067)
	}
	fingerprint := sha256.Sum256(key)
	if subtle.ConstantTimeCompare(fingerprint[:], expected) != 1 {
		return nil, fmt.Errorf("%w: operator root does not match independently pinned digest", ErrOperatorTrustV067)
	}
	return ed25519.PublicKey(key), nil
}

// NewLocalOperatorAdmitterFromPinnedRootV067 is an OPTIONAL trusted-operator
// initialization path. It verifies root pinning before the existing signature
// and snapshot checks. The older constructor remains exported; therefore this
// helper alone is not a mandatory package-wide authorization boundary.
func newLocalOperatorAdmitterFromPinnedRootV067(
	store *LocalReplayStoreV067,
	rootPath, independentlyPinnedSHA256 string,
	signed SignedOperatorTrustV067,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
) (*LocalOperatorAdmitterV067, error) {
	key, err := LoadPinnedOperatorRootV067(rootPath, independentlyPinnedSHA256)
	if err != nil {
		return nil, err
	}
	return newLocalOperatorAdmitterV067(store, key, signed, chain, pos)
}

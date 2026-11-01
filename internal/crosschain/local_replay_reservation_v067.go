package crosschain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	localReplayVersionV067        uint32 = 1
	localReplayDirectoryV067             = "crosschain-v067-local-replay"
	localReplayDomainV067                = "PRISM/CROSSCHAIN/LOCAL-REPLAY-PROOF/V067\x00"
	localReplayMaxRecordBytesV067        = 8192
)

var (
	// This error means the proof identity has already occupied its one-shot slot.
	// It does NOT imply that the stored record is valid, final or payable.
	ErrLocalReplayReservedV067 = errors.New("v0.67 source proof reservation already exists")
	ErrLocalReplayInvalidV067  = errors.New("v0.67 invalid local reservation state")
)

// LocalReplayReservationV067 is an immutable LOCAL audit record, not authorization.
// It contains NO payout, signing, bridge, consensus-finality or nonce-consumption API.
type LocalReplayReservationV067 struct {
	Version      uint32           `json:"version"`
	ProofKey     string           `json:"proof_key"`
	IntentDigest string           `json:"intent_digest"`
	Intent       AnchorIntentV067 `json:"intent"`
}

// LocalReplayStoreV067 relies on atomic exclusive creation on one trusted local
// filesystem. Multiple instances/processes on that same filesystem are supported.
// This does not protect against a malicious storage administrator, file deletion,
// shared network filesystems, or independent nodes with different storage roots.
type LocalReplayStoreV067 struct {
	dir string
}

// NewLocalReplayStoreV067 initializes a new versioned, private storage namespace.
// dataDir must be an operator-trusted local filesystem location (Linux lab).
func NewLocalReplayStoreV067(dataDir string) (*LocalReplayStoreV067, error) {
	// The local replay ledger requires Linux private directory modes and
	// durable local filesystem semantics. Windows ACLs are not POSIX modes.
	// Until separately implemented and qualified, fail closed on every other OS.
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("v0.67 local replay: unsupported platform %s: %w", runtime.GOOS, ErrLocalReplayInvalidV067)
	}
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("v0.67 local replay: empty data directory")
	}
	base, err := filepath.Abs(strings.TrimSpace(dataDir))
	if err != nil {
		return nil, fmt.Errorf("v0.67 local replay: resolve data directory: %w", err)
	}
	dir := filepath.Join(base, localReplayDirectoryV067)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("v0.67 local replay: create private directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("v0.67 local replay: unsafe storage directory or permissions: %w", errors.Join(ErrLocalReplayInvalidV067, err))
	}
	// Sync the parent so newly created namespace metadata is durable too.
	parent, err := os.Open(base)
	if err != nil {
		return nil, fmt.Errorf("v0.67 local replay: open parent directory: %w", err)
	}
	syncErr := parent.Sync()
	closeErr := parent.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return nil, fmt.Errorf("v0.67 local replay: sync parent directory: %w", err)
	}
	return &LocalReplayStoreV067{dir: dir}, nil
}

// localProofKeyV067 identifies one source PoUW proof regardless of destination,
// registry, source block, or nonce. Changing these must not bypass replay protection.
func localProofKeyV067(intent AnchorIntentV067) (string, error) {
	if err := intent.Validate(); err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(localReplayDomainV067))
	writeFieldV067(hash, intent.SourceChainID)
	writeFieldV067(hash, intent.SourceGenesisHash)
	writeFieldV067(hash, intent.JobID)
	writeFieldV067(hash, intent.ProofHash)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *LocalReplayStoreV067) recordPath(key string) string {
	return filepath.Join(s.dir, key+".json")
}

// ReserveProofV067 exclusively occupies the source-proof identity once.
// Existing, partial, malformed, or symlinked slots are NEVER overwritten.
// The caller MUST NOT use a successful reservation as proof of finality or
// as permission to register, pay, release escrow, or sign anything.
func (s *LocalReplayStoreV067) reserveProofV067(intent AnchorIntentV067) (LocalReplayReservationV067, error) {
	if s == nil || s.dir == "" {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	key, err := localProofKeyV067(intent)
	if err != nil {
		return LocalReplayReservationV067{}, err
	}
	digest, err := intent.Digest()
	if err != nil {
		return LocalReplayReservationV067{}, err
	}
	record := LocalReplayReservationV067{
		Version:      localReplayVersionV067,
		ProofKey:     key,
		IntentDigest: digest,
		Intent:       intent,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return LocalReplayReservationV067{}, err
	}
	payload = append(payload, '\n')

	// O_EXCL serializes concurrent processes/goroutines on a LOCAL filesystem.
	// A failure after file creation deliberately leaves the slot occupied.
	file, err := os.OpenFile(s.recordPath(key), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		// Existing objects, including damaged records, are never reusable.
		if _, statErr := os.Lstat(s.recordPath(key)); statErr == nil {
			return LocalReplayReservationV067{}, ErrLocalReplayReservedV067
		}
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: exclusive create: %w", err)
	}
	n, writeErr := file.Write(payload)
	if writeErr == nil && n != len(payload) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: persist reservation (slot retained): %w", err)
	}
	// Durable directory entry before acknowledging a successful reservation.
	dir, err := os.Open(s.dir)
	if err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: open directory for sync: %w", err)
	}
	syncErr := dir.Sync()
	closeErr = dir.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: sync directory (slot retained): %w", err)
	}
	return record, nil
}

// InspectProofV067 validates a previously stored record for OFFLINE auditing.
// A missing, truncated, changed, or mismatched record fails closed.
// A valid record still DOES NOT prove finality or authorize settlement.
func (s *LocalReplayStoreV067) InspectProofV067(intent AnchorIntentV067) (LocalReplayReservationV067, error) {
	if s == nil || s.dir == "" {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	key, err := localProofKeyV067(intent)
	if err != nil {
		return LocalReplayReservationV067{}, err
	}
	digest, err := intent.Digest()
	if err != nil {
		return LocalReplayReservationV067{}, err
	}
	path := s.recordPath(key)
	info, err := os.Lstat(path)
	if err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: reservation unavailable: %w", errors.Join(ErrLocalReplayInvalidV067, err))
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > localReplayMaxRecordBytesV067 {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	file, err := os.Open(path)
	if err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: read error: %w", errors.Join(ErrLocalReplayInvalidV067, err))
	}
	payload, readErr := io.ReadAll(io.LimitReader(file, localReplayMaxRecordBytesV067+1))
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil || len(payload) > localReplayMaxRecordBytesV067 {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var record LocalReplayReservationV067
	if err := decoder.Decode(&record); err != nil {
		return LocalReplayReservationV067{}, fmt.Errorf("v0.67 local replay: malformed record: %w", ErrLocalReplayInvalidV067)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	recordedKey, keyErr := localProofKeyV067(record.Intent)
	recordedDigest, digestErr := record.Intent.Digest()
	if keyErr != nil || digestErr != nil || record.Version != localReplayVersionV067 ||
		record.ProofKey != key || recordedKey != key ||
		record.IntentDigest != recordedDigest || record.IntentDigest != digest {
		return LocalReplayReservationV067{}, ErrLocalReplayInvalidV067
	}
	return record, nil
}

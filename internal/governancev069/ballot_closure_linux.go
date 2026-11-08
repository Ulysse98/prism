//go:build linux

package governancev069

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

const localBallotClosureFilename = ".prism-advisory-closed.v1"
const maxLocalClosureRecordSize = 4096

// localAdvisoryGate obtains an exclusive flock on the directory itself: no
// extra lock file is needed and existing 1B.5 audits remain strict. Flock
// only coordinates cooperating processes on a TRUSTED LOCAL Linux filesystem.
func localAdvisoryGate(dir string) (*os.File, error) {
	if err := rejectSymlinkComponents(dir); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open advisory store gate: %w", err)
	}
	f := os.NewFile(uintptr(fd), "advisory-store-gate")
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		f.Close()
		return nil, err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFDIR || st.Mode&0777 != 0700 || st.Uid != uint32(os.Geteuid()) {
		f.Close()
		return nil, errors.New("untrusted advisory store gate owner, type or permissions")
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock advisory directory: %w", err)
	}
	// A named directory replaced by a concurrent operator would invalidate
	// the path-based legacy writer; compare identities after taking the lock.
	var pathSt syscall.Stat_t
	if err := syscall.Lstat(dir, &pathSt); err != nil || st.Dev != pathSt.Dev || st.Ino != pathSt.Ino {
		f.Close()
		return nil, errors.New("advisory directory changed during lock acquisition")
	}
	return f, nil
}

// gateHasClosure fails closed on ANY existing or malformed closing entry.
// Even an incomplete O_EXCL file after power loss prevents further votes.
func gateHasClosure(dirFD int) (bool, error) {
	fd, err := syscall.Openat(dirFD, localBallotClosureFilename, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err == nil {
		syscall.Close(fd)
		return true, nil
	}
	if errors.Is(err, syscall.ENOENT) {
		return false, nil
	}
	return true, fmt.Errorf("closure marker cannot be safely inspected: %w", err)
}

func readLocalClosureFromFD(dirFD int) (AdvisoryBallotClosure, error) {
	var zero AdvisoryBallotClosure
	fd, err := syscall.Openat(dirFD, localBallotClosureFilename, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return zero, fmt.Errorf("open closure record: %w", err)
	}
	f := os.NewFile(uintptr(fd), "local-advisory-closure")
	defer f.Close()
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return zero, err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Mode&0777 != 0600 || st.Uid != uint32(os.Geteuid()) || st.Nlink != 1 || st.Size <= 0 || st.Size > maxLocalClosureRecordSize {
		return zero, errors.New("unsafe closure record type, mode, owner, link count or length")
	}
	payload, err := io.ReadAll(io.LimitReader(f, maxLocalClosureRecordSize+1))
	if err != nil {
		return zero, err
	}
	var c AdvisoryBallotClosure
	if err := json.Unmarshal(payload, &c); err != nil {
		return zero, fmt.Errorf("invalid closure JSON: %w", err)
	}
	canonical, err := json.Marshal(c)
	if err != nil {
		return zero, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(payload, canonical) || c.Version != AdvisoryBallotClosureVersion {
		return zero, errors.New("noncanonical closure record")
	}
	if !isLowerSHA256(c.ClosureSHA256) || !isLowerSHA256(c.ReservationAuditSHA256) ||
		!isLowerSHA256(c.TallyReceiptSHA256) || !isLowerSHA256(c.SnapshotSHA256) ||
		!isLowerSHA256(c.BallotSetSHA256) || !isLowerSHA256(c.ProposalSHA256) ||
		!isLowerSHA256(c.GenesisSHA256) {
		return zero, errors.New("malformed closure commitment fields")
	}
	digest := localBallotClosureCommitment(c)
	if c.ClosureSHA256 != fmt.Sprintf("%x", digest[:]) {
		return zero, errors.New("closure record SHA-256 mismatch")
	}
	return c, nil
}

// SealLocalAdvisoryVoteStore obtains the same interprocess directory lock as
// the revised 1B.2 writer, audits exactly the supplied independently-pinned
// ballot set, then atomically creates a durable, no-overwrite gate marker.
// The operator, not this unsigned function, is responsible for authorization
// and for choosing the canonical close height / pins. NO auto-reopen exists.
func SealLocalAdvisoryVoteStore(store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, snapshotPin, ballotPin string, proposal ProposalIntent, trusted Context, ballots []SignedAdvisoryVote) (AdvisoryBallotClosure, error) {
	var zero AdvisoryBallotClosure
	if store == nil {
		return zero, errors.New("nil store")
	}
	gate, err := localAdvisoryGate(store.dir)
	if err != nil {
		return zero, err
	}
	defer gate.Close()
	exists, err := gateHasClosure(int(gate.Fd()))
	if err != nil {
		return zero, err
	}
	if exists {
		return zero, ErrAdvisoryBallotClosed
	}
	audit, err := AuditLocalAdvisoryReservations(store, receipt, snapshot, snapshotPin, ballotPin, proposal, trusted, ballots)
	if err != nil {
		return zero, fmt.Errorf("pre-closure reservation audit: %w", err)
	}
	closure := makeLocalBallotClosure(receipt, audit)
	payload, err := json.Marshal(closure)
	if err != nil {
		return zero, err
	}
	payload = append(payload, '\n')
	if len(payload) > maxLocalClosureRecordSize {
		return zero, errors.New("closure record too large")
	}
	fd, err := syscall.Openat(int(gate.Fd()), localBallotClosureFilename, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if errors.Is(err, syscall.EEXIST) {
		return zero, ErrAdvisoryBallotClosed
	}
	if err != nil {
		return zero, fmt.Errorf("create closure gate: %w", err)
	}
	// Once created, NEVER remove this file on write/sync failure. An ambiguous
	// crash state must continue blocking all future admissions.
	record := os.NewFile(uintptr(fd), "new-local-advisory-closure")
	n, writeErr := record.Write(payload)
	if writeErr == nil && n != len(payload) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = record.Sync()
	}
	closeErr := record.Close()
	if writeErr != nil {
		return zero, fmt.Errorf("closure marker left fail-closed after write error: %w", writeErr)
	}
	if closeErr != nil {
		return zero, fmt.Errorf("closure marker left fail-closed after close error: %w", closeErr)
	}
	if err := gate.Sync(); err != nil {
		return zero, fmt.Errorf("closure marker exists but directory sync failed: %w", err)
	}
	return closure, nil
}

// VerifySealedLocalAdvisoryVoteStore re-audits the actual local vote records
// and compares the durable closure marker to independently pinned inputs.
// It acquires the same admission gate to prevent races with cooperating writers.
func VerifySealedLocalAdvisoryVoteStore(store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, snapshotPin, ballotPin string, proposal ProposalIntent, trusted Context, ballots []SignedAdvisoryVote) (AdvisoryBallotClosure, error) {
	var zero AdvisoryBallotClosure
	if store == nil {
		return zero, errors.New("nil store")
	}
	gate, err := localAdvisoryGate(store.dir)
	if err != nil {
		return zero, err
	}
	defer gate.Close()
	actual, err := readLocalClosureFromFD(int(gate.Fd()))
	if err != nil {
		return zero, err
	}
	audit, err := AuditLocalAdvisoryReservations(store, receipt, snapshot, snapshotPin, ballotPin, proposal, trusted, ballots)
	if err != nil {
		return zero, err
	}
	expected := makeLocalBallotClosure(receipt, audit)
	if actual != expected {
		return zero, errors.New("local closure record differs from re-audited pinned ballots")
	}
	return actual, nil
}

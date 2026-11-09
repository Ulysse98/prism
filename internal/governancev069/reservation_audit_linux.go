//go:build linux

package governancev069

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

const maxLocalVoteRecordSize = 8192

// AuditLocalAdvisoryReservations audits every file in a dedicated, private,
// single-proposal Linux reservation store against the independently pinned
// canonical ballot set and its verified tally receipt. It writes no files.
// IMPORTANT: The caller MUST stop all store writers for the full audit.
// This API does not coordinate with the Phase 1B.2 writer, cannot guarantee a
// coherent snapshot under concurrency, and cannot establish global ballot
// completeness or real closure. Readiness evidence only; fail-closed.
func AuditLocalAdvisoryReservations(store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, pinnedSnapshotSHA256, pinnedBallotSetSHA256 string, proposal ProposalIntent, trusted Context, ballots []SignedAdvisoryVote) (AdvisoryReservationAudit, error) {
	var zero AdvisoryReservationAudit
	if store == nil {
		return zero, errors.New("nil local reservation store")
	}
	if len(ballots) > MaxAdvisoryVoters {
		return zero, errors.New("too many ballots")
	}
	if err := VerifyAdvisoryTallyReceipt(receipt, snapshot, pinnedSnapshotSHA256, pinnedBallotSetSHA256, proposal, trusted, ballots); err != nil {
		return zero, fmt.Errorf("tally receipt rejected: %w", err)
	}
	expected := make(map[string][]byte, len(ballots))
	for _, ballot := range ballots {
		key, err := VoteReplayKey(ballot.Vote)
		if err != nil {
			return zero, err
		}
		name := hex.EncodeToString(key[:]) + ".vote"
		voteHash, err := AdvisoryVoteCommitment(ballot.Vote)
		if err != nil {
			return zero, err
		}
		record := struct {
			Key        string             `json:"replay_key"`
			VoteSHA256 string             `json:"vote_sha256"`
			Signed     SignedAdvisoryVote `json:"signed"`
		}{hex.EncodeToString(key[:]), hex.EncodeToString(voteHash[:]), ballot}
		canonical, err := json.Marshal(record)
		if err != nil {
			return zero, err
		}
		canonical = append(canonical, '\n')
		if len(canonical) > maxLocalVoteRecordSize {
			return zero, errors.New("oversized expected vote record")
		}
		if _, exists := expected[name]; exists {
			return zero, errors.New("duplicate local replay key")
		}
		expected[name] = canonical
	}
	if err := rejectSymlinkComponents(store.dir); err != nil {
		return zero, fmt.Errorf("untrusted store path: %w", err)
	}
	fd, err := syscall.Open(store.dir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return zero, fmt.Errorf("open private store: %w", err)
	}
	dir := os.NewFile(uintptr(fd), "private-advisory-audit-dir")
	defer dir.Close()
	var dst syscall.Stat_t
	if err = syscall.Fstat(fd, &dst); err != nil {
		return zero, err
	}
	if dst.Mode&syscall.S_IFMT != syscall.S_IFDIR || dst.Mode&0777 != 0700 || dst.Uid != uint32(os.Geteuid()) {
		return zero, errors.New("untrusted local store directory ownership or permissions")
	}
	names, err := dir.Readdirnames(MaxAdvisoryVoters + 2)
	if err != nil && !errors.Is(err, io.EOF) {
		return zero, fmt.Errorf("enumerate local reservations: %w", err)
	}
	// Phase 1B.6 accepts exactly one *verified* closure marker as metadata.
	// Arbitrary extra files still fail closed. The marker is checked against
	// the recomputed audit after all vote records have been verified.
	if len(names) < len(expected) || len(names) > len(expected)+1 {
		return zero, fmt.Errorf("reservation entry count mismatch: got %d expected %d (+ optional closure)", len(names), len(expected))
	}
	seen := make(map[string]struct{}, len(expected))
	var closureSeen bool
	for _, name := range names {
		if name == localBallotClosureFilename {
			if closureSeen {
				return zero, errors.New("duplicate closure marker entry")
			}
			closureSeen = true
			continue
		}
		canonical, ok := expected[name]
		if !ok || !strings.HasSuffix(name, ".vote") {
			return zero, fmt.Errorf("unexpected reservation entry %q", name)
		}
		if _, duplicate := seen[name]; duplicate {
			return zero, errors.New("duplicate filename in directory inspection")
		}
		seen[name] = struct{}{}
		fileFD, err := syscall.Openat(fd, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
		if err != nil {
			return zero, fmt.Errorf("open reservation %q: %w", name, err)
		}
		f := os.NewFile(uintptr(fileFD), name)
		var fst syscall.Stat_t
		err = syscall.Fstat(fileFD, &fst)
		if err == nil && (fst.Mode&syscall.S_IFMT != syscall.S_IFREG || fst.Mode&0777 != 0600 || fst.Uid != uint32(os.Geteuid()) || fst.Nlink != 1 || fst.Size != int64(len(canonical))) {
			err = errors.New("untrusted vote record type, mode, owner, link count or size")
		}
		if err == nil {
			var contents []byte
			contents, err = io.ReadAll(io.LimitReader(f, maxLocalVoteRecordSize+1))
			if err == nil && !bytes.Equal(contents, canonical) {
				err = errors.New("local reservation content mismatch")
			}
		}
		closeErr := f.Close()
		if err != nil {
			return zero, fmt.Errorf("bad reservation %q: %w", name, err)
		}
		if closeErr != nil {
			return zero, closeErr
		}
	}
	if len(seen) != len(expected) {
		return zero, errors.New("local reservation set incomplete")
	}
	audit := makeAdvisoryReservationAudit(receipt, len(expected))
	if closureSeen {
		actual, err := readLocalClosureFromFD(fd)
		if err != nil {
			return zero, fmt.Errorf("invalid closure marker: %w", err)
		}
		if expectedClosure := makeLocalBallotClosure(receipt, audit); !closureBodyEqual(actual, expectedClosure) {
			return zero, errors.New("closure marker not bound to the audited local ballot set")
		}
	}
	// Old callers must still quiesce external writers; Phase 1B.6 verifier
	// provides a shared gate for cooperating 1B.6 writers and sealers.
	return audit, nil
}

func VerifyLocalAdvisoryReservationAudit(audit AdvisoryReservationAudit, store *AdvisoryVoteReplayStore, receipt AdvisoryTallyReceipt, snapshot EligibilitySnapshot, pinnedSnapshotSHA256, pinnedBallotSetSHA256 string, proposal ProposalIntent, trusted Context, ballots []SignedAdvisoryVote) error {
	recomputed, err := AuditLocalAdvisoryReservations(store, receipt, snapshot, pinnedSnapshotSHA256, pinnedBallotSetSHA256, proposal, trusted, ballots)
	if err != nil {
		return err
	}
	if recomputed != audit {
		return errors.New("local reservation audit mismatch")
	}
	return nil
}

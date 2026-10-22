//go:build linux

package finality

// This is a Linux/WSL-only anti-equivocation experiment. It is NOT a BFT
// lock, active consensus, or protection against key use outside this API.

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"prism/internal/wallet"
)

const (
	voteJournalVersionV065  = 1
	voteJournalFilenameV065 = "votes-v065.json"
	voteJournalLocknameV065 = ".votes-v065.lock"
	voteJournalMaxBytesV065 = 4 << 20
)

type voteJournalV065 struct {
	Version int          `json:"version"`
	Votes   []SignedVote `json:"votes"`
}

type voteSlotV065 struct {
	ChainID     string
	GenesisHash string
	Height      uint64
	Round       uint64
	Step        string
	Address     string
}

func slotForVoteV065(v SignedVote) voteSlotV065 {
	return voteSlotV065{
		ChainID: v.Context.ChainID, GenesisHash: v.Context.GenesisHash,
		Height: v.Context.Height, Round: v.Context.Round,
		Step: v.Context.Step, Address: v.Address,
	}
}

func verifyStoredVoteV065(v SignedVote) error {
	if err := validateContext(v.Context); err != nil {
		return err
	}
	publicKey, err := wallet.DecodePublicKey(v.PublicKey)
	if err != nil || hex.EncodeToString(publicKey) != v.PublicKey {
		return fmt.Errorf("noncanonical public key: %v", err)
	}
	if wallet.AddressFromPublicKey(publicKey) != v.Address {
		return errors.New("stored vote address/public key mismatch")
	}
	sig, err := hex.DecodeString(v.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("invalid stored signature encoding")
	}
	payload, err := signingPayload(v)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, sig) {
		return errors.New("invalid stored vote signature")
	}
	return nil
}

func readVoteJournalV065(path string) (voteJournalV065, error) {
	journal := voteJournalV065{Version: voteJournalVersionV065}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return journal, nil
	}
	if err != nil {
		return journal, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > voteJournalMaxBytesV065 {
		return journal, errors.New("unsafe vote journal file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return journal, err
	}
	if len(data) == 0 {
		return journal, errors.New("empty vote journal; refusing to reset")
	}
	if err := json.Unmarshal(data, &journal); err != nil {
		return journal, fmt.Errorf("corrupted vote journal: %w", err)
	}
	if journal.Version != voteJournalVersionV065 {
		return journal, errors.New("unsupported vote journal version")
	}
	if len(journal.Votes) == 0 {
		return journal, errors.New("journal reset or missing recorded votes")
	}
	seen := make(map[voteSlotV065]struct{}, len(journal.Votes))
	for _, vote := range journal.Votes {
		if err := verifyStoredVoteV065(vote); err != nil {
			return journal, fmt.Errorf("invalid stored vote: %w", err)
		}
		slot := slotForVoteV065(vote)
		if _, exists := seen[slot]; exists {
			return journal, errors.New("duplicate journal signing slot")
		}
		seen[slot] = struct{}{}
	}
	return journal, nil
}

func persistVoteJournalV065(dir, path string, journal voteJournalV065) error {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > voteJournalMaxBytesV065 {
		return errors.New("vote journal size limit reached")
	}
	tmp, err := os.CreateTemp(dir, ".votes-v065-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	folder, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := folder.Sync()
	closeErr := folder.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// SignVoteOnceV065 serializes signers using a Linux flock and stores a complete
// signed vote atomically before returning it. The signing slot deliberately
// excludes BlockHash and ValidatorSetHash to fail closed on conflicting votes
// for the same (network, genesis, height, round, step, validator).
//
// SECURITY SCOPE: one trusted Linux filesystem directory, one signing API.
// Not a consensus lock across rounds. Not secure if this journal is deleted,
// rolled back, edited by a privileged actor, or the key is used elsewhere.
func SignVoteOnceV065(dir string, ctx VoteContext, key ed25519.PrivateKey) (SignedVote, error) {
	if dir == "" {
		return SignedVote{}, errors.New("missing journal directory")
	}
	if err := validateContext(ctx); err != nil {
		return SignedVote{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return SignedVote{}, errors.New("invalid private key")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return SignedVote{}, err
	}
	stat, err := os.Lstat(dir)
	if err != nil || !stat.IsDir() || stat.Mode().Perm()&0077 != 0 {
		return SignedVote{}, fmt.Errorf("unsafe vote journal directory: %v", err)
	}

	lockPath := filepath.Join(dir, voteJournalLocknameV065)
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return SignedVote{}, err
	}
	defer lock.Close()
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return SignedVote{}, errors.New("unsafe vote journal lock file")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return SignedVote{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	path := filepath.Join(dir, voteJournalFilenameV065)
	journal, err := readVoteJournalV065(path)
	if err != nil {
		return SignedVote{}, err
	}

	pub := key.Public().(ed25519.PublicKey)
	wanted := voteSlotV065{
		ChainID: ctx.ChainID, GenesisHash: ctx.GenesisHash,
		Height: ctx.Height, Round: ctx.Round, Step: ctx.Step,
		Address: wallet.AddressFromPublicKey(pub),
	}
	for _, prior := range journal.Votes {
		if slotForVoteV065(prior) != wanted {
			continue
		}
		if prior.Context != ctx {
			return SignedVote{}, fmt.Errorf("conflicting vote already durably recorded for height=%d round=%d step=%s", ctx.Height, ctx.Round, ctx.Step)
		}
		return prior, nil // exact idempotent retry, including original signature
	}

	vote, err := SignVote(ctx, key)
	if err != nil {
		return SignedVote{}, err
	}
	journal.Votes = append(journal.Votes, vote)
	if err := persistVoteJournalV065(dir, path, journal); err != nil {
		return SignedVote{}, fmt.Errorf("failed to persist signed vote: %w", err)
	}
	return vote, nil
}

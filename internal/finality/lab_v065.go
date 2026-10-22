// Package finality is an isolated research lab, NOT active PRISM consensus.
// A weighted vote quorum alone does not provide Byzantine finality.
package finality

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

const (
	stepPrevote   = "prevote"
	stepPrecommit = "precommit"
	voteDomain    = "prism-finality-lab-v065/vote/v1\x00"
	setDomain     = "prism-finality-lab-v065/validator-set/v1\x00"
)

type VoteContext struct {
	ChainID          string `json:"chain_id"`
	GenesisHash      string `json:"genesis_hash"`
	Height           uint64 `json:"height"`
	Round            uint64 `json:"round"`
	Step             string `json:"step"`
	BlockHash        string `json:"block_hash"`
	ValidatorSetHash string `json:"validator_set_hash"`
	// Nil explicitly distinguishes a NIL vote from a vote for a block.
	// omitempty preserves v0.65 block-vote signing bytes and stored votes.
	Nil bool `json:"nil,omitempty"`
}

type SignedVote struct {
	Context   VoteContext `json:"context"`
	Address   string      `json:"address"`
	PublicKey string      `json:"public_key"`
	Signature string      `json:"signature"`
}

type Certificate struct {
	Context VoteContext  `json:"context"`
	Votes   []SignedVote `json:"votes"`
}

func validHash(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size && hex.EncodeToString(raw) == value
}

func validateContext(ctx VoteContext) error {
	if ctx.ChainID == "" || !validHash(ctx.GenesisHash) || !validHash(ctx.ValidatorSetHash) || ctx.Height == 0 {
		return fmt.Errorf("incomplete or noncanonical vote context")
	}
	// Exactly one choice is permitted: an explicit NIL or a canonical block hash.
	if ctx.Nil {
		if ctx.BlockHash != "" {
			return fmt.Errorf("NIL vote must not carry a block hash")
		}
	} else if !validHash(ctx.BlockHash) {
		return fmt.Errorf("non-NIL vote requires a canonical block hash")
	}
	if ctx.Step != stepPrevote && ctx.Step != stepPrecommit {
		return fmt.Errorf("unsupported vote step: %q", ctx.Step)
	}
	return nil
}

// ValidatorSetHash commits a sorted, strictly validated stake snapshot.
// A caller MUST independently authenticate the snapshot for this height.
func ValidatorSetHash(validators []consensus.Validator) (string, error) {
	if len(validators) == 0 {
		return "", fmt.Errorf("empty validator snapshot")
	}
	ordered := append([]consensus.Validator(nil), validators...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Address < ordered[j].Address
	})
	var total uint64
	for i, v := range ordered {
		if v.Address == "" || v.Address == "GENESIS" || v.Stake == 0 {
			return "", fmt.Errorf("invalid validator at index %d", i)
		}
		if i > 0 && v.Address == ordered[i-1].Address {
			return "", fmt.Errorf("duplicate validator %s", v.Address)
		}
		if math.MaxUint64-total < v.Stake {
			return "", fmt.Errorf("validator stake total overflow")
		}
		total += v.Stake
	}
	serialized, err := json.Marshal(ordered)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte(setDomain), serialized...))
	return hex.EncodeToString(digest[:]), nil
}

func signingPayload(vote SignedVote) ([]byte, error) {
	unsigned := struct {
		Context   VoteContext `json:"context"`
		Address   string      `json:"address"`
		PublicKey string      `json:"public_key"`
	}{vote.Context, vote.Address, vote.PublicKey}
	serialized, err := json.Marshal(unsigned)
	if err != nil {
		return nil, err
	}
	return append([]byte(voteDomain), serialized...), nil
}

// SignVote signs a proposed vote; this function is NOT a double-sign guard.
func SignVote(ctx VoteContext, key ed25519.PrivateKey) (SignedVote, error) {
	if err := validateContext(ctx); err != nil {
		return SignedVote{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return SignedVote{}, fmt.Errorf("invalid Ed25519 private key size")
	}
	pub := key.Public().(ed25519.PublicKey)
	vote := SignedVote{
		Context:   ctx,
		Address:   wallet.AddressFromPublicKey(pub),
		PublicKey: hex.EncodeToString(pub),
	}
	payload, err := signingPayload(vote)
	if err != nil {
		return SignedVote{}, err
	}
	vote.Signature = hex.EncodeToString(ed25519.Sign(key, payload))
	return vote, nil
}

// VerifyQuorum checks the signatures and strict >2/3 stake of a trusted
// snapshot for one (chain, genesis, height, round, step, block, set) context.
// It is NOT a finality decision: locks, rounds and slashing are absent.
func VerifyQuorum(validators []consensus.Validator, cert Certificate) (uint64, uint64, error) {
	expected, err := ValidatorSetHash(validators)
	if err != nil {
		return 0, 0, err
	}
	if err := validateContext(cert.Context); err != nil {
		return 0, 0, err
	}
	if cert.Context.ValidatorSetHash != expected {
		return 0, 0, fmt.Errorf("validator snapshot hash mismatch")
	}
	weights := make(map[string]uint64, len(validators))
	var total uint64
	for _, v := range validators {
		weights[v.Address] = v.Stake
		total += v.Stake
	}
	seen := make(map[string]struct{}, len(cert.Votes))
	var signed uint64
	for _, vote := range cert.Votes {
		if vote.Context != cert.Context {
			return 0, total, fmt.Errorf("mixed vote context")
		}
		stake, found := weights[vote.Address]
		if !found {
			return 0, total, fmt.Errorf("unknown voter %s", vote.Address)
		}
		if _, duplicate := seen[vote.Address]; duplicate {
			return 0, total, fmt.Errorf("duplicate vote from %s", vote.Address)
		}
		seen[vote.Address] = struct{}{}
		publicKey, err := wallet.DecodePublicKey(vote.PublicKey)
		if err != nil {
			return 0, total, err
		}
		if hex.EncodeToString(publicKey) != vote.PublicKey || wallet.AddressFromPublicKey(publicKey) != vote.Address {
			return 0, total, fmt.Errorf("voter public key mismatch")
		}
		signature, err := hex.DecodeString(vote.Signature)
		if err != nil || len(signature) != ed25519.SignatureSize {
			return 0, total, fmt.Errorf("invalid vote signature encoding")
		}
		payload, err := signingPayload(vote)
		if err != nil {
			return 0, total, err
		}
		if !ed25519.Verify(publicKey, payload, signature) {
			return 0, total, fmt.Errorf("invalid vote signature")
		}
		signed += stake // No overflow: each unique voter belongs to validated snapshot.
	}
	required := 2*(total/3) + 2*(total%3)/3 + 1 // floor(2*total/3)+1, overflow-safe.
	if signed < required {
		return signed, total, fmt.Errorf("insufficient voting stake: signed=%d required=%d total=%d", signed, required, total)
	}
	return signed, total, nil
}

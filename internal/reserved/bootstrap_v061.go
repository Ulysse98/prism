package reserved

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"

	"prism/internal/consensus"
	"prism/internal/wallet"
)

// GovernanceBootstrap is a signed validator vote to initialize an empty
// reserved authority pool. This object by itself DOES NOT change consensus
// state, authorize an allocation or mint PRISM.
//
// A later consensus integration must check that the pool was previously empty,
// activate only at the agreed height and prevent a second bootstrap.
type GovernanceBootstrap struct {
	ID               string                 `json:"id"`
	ChainID          string                 `json:"chain_id"`
	GenesisHash      string                 `json:"genesis_hash"`
	Pool             consensus.ReservedPool `json:"pool"`
	Authorities      []string               `json:"authorities"`
	Threshold        uint32                 `json:"threshold"`
	Nonce            uint64                 `json:"nonce"`
	ActivationHeight uint64                 `json:"activation_height"`
	Approvals        []Approval             `json:"approvals"`
}

var prismBootstrapAddressRE = regexp.MustCompile(`^prism_[0-9a-f]{40}$`)
var prismBootstrapHashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Bootstrap intent is encoded by encoding/json on an ordered struct.
// Approvals and ID are deliberately excluded from the signed payload.
type governanceBootstrapIntent struct {
	ChainID          string                 `json:"chain_id"`
	GenesisHash      string                 `json:"genesis_hash"`
	Pool             consensus.ReservedPool `json:"pool"`
	Authorities      []string               `json:"authorities"`
	Threshold        uint32                 `json:"threshold"`
	Nonce            uint64                 `json:"nonce"`
	ActivationHeight uint64                 `json:"activation_height"`
}

func (b GovernanceBootstrap) intent() governanceBootstrapIntent {
	return governanceBootstrapIntent{
		ChainID:          b.ChainID,
		GenesisHash:      b.GenesisHash,
		Pool:             b.Pool,
		Authorities:      b.Authorities,
		Threshold:        b.Threshold,
		Nonce:            b.Nonce,
		ActivationHeight: b.ActivationHeight,
	}
}

func (b GovernanceBootstrap) calculateID() (string, error) {
	raw, err := json.Marshal(b.intent())
	if err != nil {
		return "", fmt.Errorf("encode bootstrap intent: %w", err)
	}
	payload := append([]byte("prism-governance-bootstrap-v1|"), raw...)
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:]), nil
}

func (b GovernanceBootstrap) validateIntent() error {
	if b.ChainID == "" {
		return fmt.Errorf("bootstrap chain ID missing")
	}
	if !prismBootstrapHashRE.MatchString(b.GenesisHash) {
		return fmt.Errorf("bootstrap genesis hash invalid")
	}
	if b.Pool != consensus.ReservedPoolTeam {
		return fmt.Errorf("v0.61 bootstrap only permits team pool")
	}
	if b.Nonce == 0 {
		return fmt.Errorf("bootstrap nonce must be positive")
	}
	if b.ActivationHeight < 4 {
		return fmt.Errorf("bootstrap activation height must be at least 4")
	}
	if len(b.Authorities) < 2 {
		return fmt.Errorf("bootstrap needs at least two authorities")
	}
	if b.Threshold < 2 || int(b.Threshold) > len(b.Authorities) {
		return fmt.Errorf("invalid bootstrap authority threshold")
	}
	prev := ""
	for _, address := range b.Authorities {
		if !prismBootstrapAddressRE.MatchString(address) {
			return fmt.Errorf("invalid bootstrap authority address")
		}
		if prev != "" && prev >= address {
			return fmt.Errorf("bootstrap authority list must be unique and sorted")
		}
		prev = address
	}
	expectedID, err := b.calculateID()
	if err != nil {
		return err
	}
	if b.ID != expectedID {
		return fmt.Errorf("bootstrap ID does not match signed intent")
	}
	return nil
}

func NewGovernanceBootstrap(chainID, genesisHash string, pool consensus.ReservedPool, authorities []string, threshold uint32, nonce, activationHeight uint64) (GovernanceBootstrap, error) {
	canonical := append([]string(nil), authorities...)
	sort.Strings(canonical)
	b := GovernanceBootstrap{
		ChainID: chainID, GenesisHash: genesisHash, Pool: pool,
		Authorities: canonical, Threshold: threshold, Nonce: nonce,
		ActivationHeight: activationHeight, Approvals: []Approval{},
	}
	id, err := b.calculateID()
	if err != nil {
		return GovernanceBootstrap{}, err
	}
	b.ID = id
	if err := b.validateIntent(); err != nil {
		return GovernanceBootstrap{}, err
	}
	return b, nil
}

func (b *GovernanceBootstrap) AddApproval(w *wallet.Wallet) error {
	if b == nil {
		return fmt.Errorf("nil bootstrap")
	}
	if err := b.validateIntent(); err != nil {
		return err
	}
	if w == nil || len(w.PrivateKey) != ed25519.PrivateKeySize || len(w.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid validator wallet")
	}
	if !bytes.Equal(w.PrivateKey.Public().(ed25519.PublicKey), w.PublicKey) ||
		wallet.AddressFromPublicKey(w.PublicKey) != w.Address {
		return fmt.Errorf("validator keypair or address mismatch")
	}
	for _, a := range b.Approvals {
		if a.Authorizer == w.Address {
			return fmt.Errorf("duplicate bootstrap approval")
		}
	}
	b.Approvals = append(b.Approvals, Approval{
		Authorizer: w.Address, PublicKey: w.PublicKeyHex(),
		Signature: hex.EncodeToString(ed25519.Sign(w.PrivateKey, []byte(b.ID))),
	})
	sort.Slice(b.Approvals, func(i, j int) bool { return b.Approvals[i].Authorizer < b.Approvals[j].Authorizer })
	return nil
}

// ValidateGovernanceBootstrap verifies a fixed validator stake snapshot.
// expectedHeight and expectedNonce must come from deterministic consensus state,
// NOT from values supplied by the requester. This does not apply bootstrap.
func ValidateGovernanceBootstrap(b GovernanceBootstrap, validators []consensus.Validator, expectedChainID, expectedGenesisHash string, expectedHeight, expectedNonce uint64) error {
	if err := b.validateIntent(); err != nil {
		return err
	}
	if b.ChainID != expectedChainID || b.GenesisHash != expectedGenesisHash {
		return fmt.Errorf("bootstrap network identity mismatch")
	}
	if b.ActivationHeight != expectedHeight || b.Nonce != expectedNonce {
		return fmt.Errorf("bootstrap height or nonce mismatch")
	}
	var total uint64
	stake := make(map[string]uint64, len(validators))
	for _, v := range validators {
		if !prismBootstrapAddressRE.MatchString(v.Address) || v.Stake == 0 {
			return fmt.Errorf("invalid validator entry")
		}
		if _, exists := stake[v.Address]; exists {
			return fmt.Errorf("duplicate validator address")
		}
		if total > math.MaxUint64-v.Stake {
			return fmt.Errorf("validator stake overflow")
		}
		stake[v.Address] = v.Stake
		total += v.Stake
	}
	if total == 0 {
		return fmt.Errorf("no validator stake")
	}
	var signed uint64
	prev := ""
	for _, a := range b.Approvals {
		if prev != "" && prev >= a.Authorizer {
			return fmt.Errorf("bootstrap approvals must be unique and sorted")
		}
		prev = a.Authorizer
		validatorStake, exists := stake[a.Authorizer]
		if !exists {
			return fmt.Errorf("bootstrap signer is not a validator")
		}
		publicKey, err := wallet.DecodePublicKey(a.PublicKey)
		if err != nil {
			return err
		}
		if wallet.AddressFromPublicKey(publicKey) != a.Authorizer {
			return fmt.Errorf("bootstrap signer address mismatch")
		}
		signature, err := hex.DecodeString(a.Signature)
		if err != nil || len(signature) != ed25519.SignatureSize {
			return fmt.Errorf("bootstrap signature encoding invalid")
		}
		if !ed25519.Verify(publicKey, []byte(b.ID), signature) {
			return fmt.Errorf("bootstrap signature invalid")
		}
		signed += validatorStake // bounded by already checked total: each validator may sign once.
	}
	// Strictly > 2/3, avoiding multiplication overflow in 3*signed > 2*total.
	requiredFloor := (total/3)*2 + ((total%3)*2)/3
	if signed <= requiredFloor {
		return fmt.Errorf("insufficient bootstrap signed stake: have=%d total=%d", signed, total)
	}
	return nil
}

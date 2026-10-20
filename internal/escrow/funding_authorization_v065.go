package escrow

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"prism/internal/wallet"
)

// FundingAuthorization is a purpose-specific Ed25519 funding intent.
// It is NOT an ordinary transfer, and cannot be replayed as one.
// Consensus must additionally check the canonical height, network, balance,
// and replay/nonce uniqueness before it locks any coins.
type FundingAuthorization struct {
	Funding   Funding `json:"funding"`
	Nonce     uint64  `json:"nonce"`
	PublicKey string  `json:"publicKey"`
	Signature string  `json:"signature"`
}

func fundingAuthorizationDigest(a FundingAuthorization) string {
	f := a.Funding
	payload := fmt.Sprintf(
		"prism-escrow-funding-v065|%s|%s|%s|%s|%d|%d|%d|%d|%s",
		f.ChainID, f.GenesisHash, f.JobID, f.Requester,
		f.Amount, f.FundedAtHeight, f.ExpiresAtHeight, a.Nonce, a.PublicKey,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// SignFunding signs a commitment to job, amount, expiry, nonce, and network.
// The private key stays with the caller; it is never persisted in a block.
func SignFunding(f Funding, nonce uint64, privateKey ed25519.PrivateKey) (FundingAuthorization, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return FundingAuthorization{}, fmt.Errorf("invalid funding private key")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	a := FundingAuthorization{
		Funding:   f,
		Nonce:     nonce,
		PublicKey: hex.EncodeToString(publicKey),
	}
	a.Funding.FundingTxID = fundingAuthorizationDigest(a)
	if _, err := NewFunded(a.Funding); err != nil {
		return FundingAuthorization{}, err
	}
	if wallet.AddressFromPublicKey(publicKey) != f.Requester {
		return FundingAuthorization{}, fmt.Errorf("funding key does not own requester")
	}
	a.Signature = hex.EncodeToString(ed25519.Sign(privateKey, []byte(a.Funding.FundingTxID)))
	return a, nil
}

// VerifyFunding authenticates the whole intent, not just the account.
func VerifyFunding(a FundingAuthorization) error {
	if _, err := NewFunded(a.Funding); err != nil {
		return err
	}
	if a.Funding.FundingTxID != fundingAuthorizationDigest(a) {
		return fmt.Errorf("funding authorization digest mismatch")
	}
	key, err := wallet.DecodePublicKey(a.PublicKey)
	if err != nil {
		return err
	}
	if hex.EncodeToString(key) != a.PublicKey || wallet.AddressFromPublicKey(key) != a.Funding.Requester {
		return fmt.Errorf("funding public key or requester mismatch")
	}
	signature, err := hex.DecodeString(a.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || hex.EncodeToString(signature) != a.Signature {
		return fmt.Errorf("invalid funding signature encoding")
	}
	if !ed25519.Verify(key, []byte(a.Funding.FundingTxID), signature) {
		return fmt.Errorf("invalid funding signature")
	}
	return nil
}

package wallet

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Wallet struct {
	Address    string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

func New() (*Wallet, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	return &Wallet{
		Address:    AddressFromPublicKey(publicKey),
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

func AddressFromPublicKey(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)

	// Format d'adresse prototype Prism.
	return "prism_" + hex.EncodeToString(hash[:20])
}

func (w *Wallet) PublicKeyHex() string {
	return hex.EncodeToString(w.PublicKey)
}

func DecodePublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid public key encoding: %w", err)
	}

	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size")
	}

	return ed25519.PublicKey(raw), nil
}

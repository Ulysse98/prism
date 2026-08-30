package transaction

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"prism/internal/wallet"
)

type Transaction struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    uint64 `json:"amount"`
	Nonce     uint64 `json:"nonce"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
}

func New(
	from string,
	to string,
	amount uint64,
	nonce uint64,
	publicKey string,
) Transaction {

	tx := Transaction{
		From:      from,
		To:        to,
		Amount:    amount,
		Nonce:     nonce,
		PublicKey: publicKey,
	}

	tx.ID = CalculateID(tx)

	return tx
}

func NewGenesis(
	to string,
	amount uint64,
) Transaction {

	tx := Transaction{
		From:   "GENESIS",
		To:     to,
		Amount: amount,
		Nonce:  0,
	}

	tx.ID = CalculateID(tx)

	return tx
}

func signingPayload(tx Transaction) string {
	return fmt.Sprintf(
		"%s|%s|%d|%d|%s",
		tx.From,
		tx.To,
		tx.Amount,
		tx.Nonce,
		tx.PublicKey,
	)
}

func CalculateID(tx Transaction) string {
	hash := sha256.Sum256(
		[]byte(signingPayload(tx)),
	)

	return hex.EncodeToString(hash[:])
}

func (tx *Transaction) Sign(
	privateKey ed25519.PrivateKey,
) error {

	if tx.From == "GENESIS" {
		return fmt.Errorf("GENESIS transactions cannot be signed")
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)

	expectedPublicKey, err := wallet.DecodePublicKey(tx.PublicKey)
	if err != nil {
		return err
	}

	if !bytes.Equal(publicKey, expectedPublicKey) {
		return fmt.Errorf(
			"private key does not match transaction public key",
		)
	}

	tx.ID = CalculateID(*tx)

	signature := ed25519.Sign(
		privateKey,
		[]byte(tx.ID),
	)

	tx.Signature = hex.EncodeToString(signature)

	return nil
}

func ValidateSigned(tx Transaction) error {
	if tx.From == "" {
		return fmt.Errorf("transaction sender cannot be empty")
	}

	if tx.From == "GENESIS" {
		return fmt.Errorf(
			"GENESIS transactions are only allowed in block 0",
		)
	}

	if tx.To == "" {
		return fmt.Errorf("transaction recipient cannot be empty")
	}

	if tx.From == tx.To {
		return fmt.Errorf(
			"sender and recipient cannot be identical",
		)
	}

	if tx.Amount == 0 {
		return fmt.Errorf(
			"transaction amount must be greater than zero",
		)
	}

	if tx.PublicKey == "" {
		return fmt.Errorf("missing public key")
	}

	if tx.Signature == "" {
		return fmt.Errorf("missing signature")
	}

	if tx.ID != CalculateID(tx) {
		return fmt.Errorf("invalid transaction ID")
	}

	publicKey, err := wallet.DecodePublicKey(tx.PublicKey)
	if err != nil {
		return err
	}

	address := wallet.AddressFromPublicKey(publicKey)

	if address != tx.From {
		return fmt.Errorf(
			"public key does not own sender address",
		)
	}

	signature, err := hex.DecodeString(tx.Signature)
	if err != nil {
		return fmt.Errorf(
			"invalid signature encoding: %w",
			err,
		)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature size")
	}

	if !ed25519.Verify(
		publicKey,
		[]byte(tx.ID),
		signature,
	) {
		return fmt.Errorf("invalid Ed25519 signature")
	}

	return nil
}

func ValidateGenesis(tx Transaction) error {
	if tx.From != "GENESIS" {
		return fmt.Errorf(
			"genesis transaction must originate from GENESIS",
		)
	}

	if tx.To == "" {
		return fmt.Errorf(
			"genesis recipient cannot be empty",
		)
	}

	if tx.Amount == 0 {
		return fmt.Errorf(
			"genesis amount must be greater than zero",
		)
	}

	if tx.PublicKey != "" {
		return fmt.Errorf(
			"genesis transaction cannot contain a public key",
		)
	}

	if tx.Signature != "" {
		return fmt.Errorf(
			"genesis transaction cannot contain a signature",
		)
	}

	if tx.ID != CalculateID(tx) {
		return fmt.Errorf(
			"invalid genesis transaction ID",
		)
	}

	return nil
}

package transaction

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// VersionV2 uses an explicitly network-bound, domain-separated transaction ID.
// Version 0 remains the existing v1 wire format for historical compatibility.
const VersionV2 uint8 = 2

// NewV2 constructs a transaction that commits to the configured Prism network.
// The caller signs it locally with Transaction.Sign; no secret key enters the API.
func NewV2(from, to string, amount, nonce uint64, publicKey, chainID, genesisHash string) (Transaction, error) {
	tx := Transaction{
		Version:     VersionV2,
		From:        from,
		To:          to,
		Amount:      amount,
		Nonce:       nonce,
		PublicKey:   publicKey,
		ChainID:     chainID,
		GenesisHash: genesisHash,
	}
	if err := validateTransactionVersion(tx); err != nil {
		return Transaction{}, err
	}
	tx.ID = CalculateID(tx)
	return tx, nil
}

// Only fixed-format strings participate in the delimited v2 payload.
// No attacker-controlled delimiter may occur in a valid v2 transaction.
func validatePrismHex(value, prefix string, byteCount int) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	data := value[len(prefix):]
	if len(data) != byteCount*2 {
		return false
	}
	decoded, err := hex.DecodeString(data)
	return err == nil && len(decoded) == byteCount && hex.EncodeToString(decoded) == data
}

func validateTransactionVersion(tx Transaction) error {
	switch tx.Version {
	case 0:
		if tx.ChainID != "" || tx.GenesisHash != "" {
			return fmt.Errorf("legacy transaction cannot contain v2 network context")
		}
	case VersionV2:
		if !validatePrismHex(tx.ChainID, "prism-", 8) {
			return fmt.Errorf("invalid v2 transaction chain ID")
		}
		if !validatePrismHex(tx.GenesisHash, "", 32) {
			return fmt.Errorf("invalid v2 transaction genesis hash")
		}
		if !validatePrismHex(tx.From, "prism_", 20) ||
			!validatePrismHex(tx.To, "prism_", 20) {
			return fmt.Errorf("invalid v2 transaction Prism address")
		}
	default:
		return fmt.Errorf("unsupported transaction version %d", tx.Version)
	}
	return nil
}

func signingPayloadV2(tx Transaction) string {
	return fmt.Sprintf(
		"prism-transaction-v2|%s|%s|%s|%s|%d|%d|%s",
		tx.ChainID, tx.GenesisHash, tx.From, tx.To,
		tx.Amount, tx.Nonce, tx.PublicKey,
	)
}

// ValidateOnChain rejects v2 transactions signed for a different chain.
// Legacy v1 remains permitted for historical and non-upgraded producers;
// a future consensus upgrade must define its network-wide sunset height.
func ValidateOnChain(tx Transaction, chainID, genesisHash string) error {
	if err := ValidateSigned(tx); err != nil {
		return err
	}
	if tx.Version == VersionV2 &&
		(tx.ChainID != chainID || tx.GenesisHash != genesisHash) {
		return fmt.Errorf("v2 transaction is signed for another Prism network")
	}
	return nil
}

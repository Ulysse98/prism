package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// calculateGovernanceBootstrapBlockHashV061 commits all block fields,
// including validator approvals. This hash function does NOT authorize or
// activate a bootstrap. Block acceptance stays fail-closed until the next
// consensus/replay integration is complete.
func calculateGovernanceBootstrapBlockHashV061(block Block) string {
	unsigned := block
	unsigned.Hash = ""
	data, err := json.Marshal(unsigned)
	if err != nil {
		panic(err)
	}
	payload := append([]byte("prism-governance-bootstrap-block-v1|"), data...)
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

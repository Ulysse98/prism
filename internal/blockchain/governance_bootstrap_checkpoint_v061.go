package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"prism/internal/consensus"
)

// v0.61 is a chain-specific, explicitly coordinated protocol upgrade.
// These values are a PROPOSED trust anchor for the existing Prism network,
// not a new genesis and not an automatically authorized hard fork.
// All validators must agree to the same immutable manifest before deployment.
const (
	v061CheckpointChainID            = "prism-ceea53aa87bcae01"
	v061CheckpointGenesisHash        = "807567a62284fb7990ab6a3d75c0a8255928a5d3b8681ec7fb86eac8615c5f85"
	v061CheckpointBlockHash          = "d5f0ae89725004f3d99f66a5d8b5b2d5061b941980c3e59c0751ca01cd0da521"
	v061CheckpointFingerprint        = "ad0c27569c5858683c6c3744ab297d3a808a76590bb3630e4d2ca88e92b95966"
	v061CheckpointHeight      uint64 = 5
)

// pinnedCheckpointValidatorsV061 returns an independent copy of the validator
// weights proposed for the height-5 checkpoint. It MUST NOT read current
// LockedStakes or a peer-provided validator list during historical replay.
// Callers must independently validate the underlying chain's block hashes.
func pinnedCheckpointValidatorsV061(chainID, genesisHash, block5Hash string) ([]consensus.Validator, error) {
	if chainID != v061CheckpointChainID ||
		genesisHash != v061CheckpointGenesisHash ||
		block5Hash != v061CheckpointBlockHash {
		return nil, fmt.Errorf("v0.61 bootstrap checkpoint identity is not approved for this chain")
	}

	// Addresses are sorted lexicographically. These are public addresses,
	// never private keys or signing material.
	validators := []consensus.Validator{
		{Address: "prism_43c25b9e3bb5ccddb14c9e68a5565427f4396bdd", Stake: 300},
		{Address: "prism_6d62655b9419e78251d840ad0c2d962eb544a57f", Stake: 200},
		{Address: "prism_86069325d62d6362b2930029f5f3b5b88e3edde9", Stake: 500},
	}

	if !sort.SliceIsSorted(validators, func(i, j int) bool {
		return validators[i].Address < validators[j].Address
	}) {
		return nil, fmt.Errorf("v0.61 pinned validators are not sorted")
	}

	fields := []string{
		"prism-v061-checkpoint-candidate-v1", chainID, genesisHash, block5Hash,
	}
	var total uint64
	for i, validator := range validators {
		if validator.Stake == 0 || (i > 0 && validators[i-1].Address == validator.Address) {
			return nil, fmt.Errorf("invalid v0.61 pinned validator manifest")
		}
		total += validator.Stake
		fields = append(fields, fmt.Sprintf("%s:%d", validator.Address, validator.Stake))
	}
	if total != 1000 {
		return nil, fmt.Errorf("unexpected v0.61 checkpoint total stake")
	}

	digest := sha256.Sum256([]byte(strings.Join(fields, "|")))
	if hex.EncodeToString(digest[:]) != v061CheckpointFingerprint {
		return nil, fmt.Errorf("v0.61 pinned checkpoint fingerprint mismatch")
	}
	return validators, nil
}

package crosschain

import (
	"bytes"
	"encoding/json"
	"fmt"

	"prism/internal/blockchain"
	"prism/internal/consensus"
)

// localOperatorSnapshotMaxBytesV067 bounds offline snapshot materialization.
// Oversized chains are rejected: this is a local-lab admission boundary, not
// a production snapshot streaming implementation.
const localOperatorSnapshotMaxBytesV067 = 64 << 20

// snapshotOperatorSourceV067 makes private copies of the operator-provided
// chain and validator set. The trusted caller MUST prevent concurrent source
// mutation for the entire duration of this function; JSON encoding alone is
// NOT a read lock. Neither the snapshot nor the checkpoint proves finality.
func snapshotOperatorSourceV067(
	source *blockchain.Blockchain,
	validators *consensus.ProofOfStake,
) (*blockchain.Blockchain, *consensus.ProofOfStake, error) {
	if source == nil || validators == nil {
		return nil, nil, fmt.Errorf("%w: nil snapshot source", ErrOperatorTrustV067)
	}

	chainJSON, err := json.Marshal(source)
	if err != nil || len(chainJSON) > localOperatorSnapshotMaxBytesV067 {
		return nil, nil, fmt.Errorf("%w: encode chain snapshot: %v", ErrOperatorTrustV067, err)
	}
	posJSON, err := json.Marshal(validators)
	if err != nil || len(posJSON) > localOperatorSnapshotMaxBytesV067 {
		return nil, nil, fmt.Errorf("%w: encode validator snapshot: %v", ErrOperatorTrustV067, err)
	}

	var chainCopy blockchain.Blockchain
	var posCopy consensus.ProofOfStake
	if err := json.Unmarshal(chainJSON, &chainCopy); err != nil {
		return nil, nil, fmt.Errorf("%w: decode chain snapshot: %v", ErrOperatorTrustV067, err)
	}
	if err := json.Unmarshal(posJSON, &posCopy); err != nil {
		return nil, nil, fmt.Errorf("%w: decode validator snapshot: %v", ErrOperatorTrustV067, err)
	}

	// Refuse non-round-trippable public representations. In addition to this
	// gate, production deployment requires auditing every consensus-relevant
	// field for JSON omission or custom serialization.
	chainAgain, err := json.Marshal(&chainCopy)
	if err != nil || !bytes.Equal(chainJSON, chainAgain) {
		return nil, nil, fmt.Errorf("%w: chain snapshot did not round-trip", ErrOperatorTrustV067)
	}
	posAgain, err := json.Marshal(&posCopy)
	if err != nil || !bytes.Equal(posJSON, posAgain) {
		return nil, nil, fmt.Errorf("%w: validator snapshot did not round-trip", ErrOperatorTrustV067)
	}
	if len(chainCopy.Blocks) < 2 || len(posCopy.Validators) == 0 || !chainCopy.ValidateChain(&posCopy) {
		return nil, nil, fmt.Errorf("%w: invalid independent source snapshot", ErrOperatorTrustV067)
	}
	return &chainCopy, &posCopy, nil
}

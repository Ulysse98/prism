package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// calculateQueuedGovernanceBlockHashV2 calculates the canonical block hash
// for blocks containing governed reserved-transfer objects.
//
// Historical blocks without reserved-transfer objects keep their previous
// hash domains unchanged.
func calculateQueuedGovernanceBlockHashV2(
	block Block,
	transactionData []byte,
	usefulWorkData []byte,
) string {
	humanityData, err := json.Marshal(
		block.Humanity,
	)
	if err != nil {
		panic(err)
	}

	claimData, err := json.Marshal(
		block.ParticipationClaims,
	)
	if err != nil {
		panic(err)
	}

	authorizationData, err := json.Marshal(
		block.ReservedAuthorizations,
	)
	if err != nil {
		panic(err)
	}

	grantData, err := json.Marshal(
		block.ReservedGrants,
	)
	if err != nil {
		panic(err)
	}

	revocationData, err := json.Marshal(
		block.ReservedRevocations,
	)
	if err != nil {
		panic(err)
	}

	authorityChangeData, err := json.Marshal(
		block.AuthorityChanges,
	)
	if err != nil {
		panic(err)
	}

	authorityProposalData, err := json.Marshal(
		block.AuthorityProposals,
	)
	if err != nil {
		panic(err)
	}

	authorityExecutionData, err := json.Marshal(
		block.AuthorityExecutions,
	)
	if err != nil {
		panic(err)
	}

	transferProposalData, err := json.Marshal(
		block.ReservedTransferProposals,
	)
	if err != nil {
		panic(err)
	}

	transferExecutionData, err := json.Marshal(
		block.ReservedTransferExecutions,
	)
	if err != nil {
		panic(err)
	}

	payload := fmt.Sprintf(
		"queued-governance-block-v2|%d|%s|%s|%s|%d|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		block.Height,
		block.Timestamp.UTC().Format(
			time.RFC3339Nano,
		),
		block.PreviousHash,
		block.Proposer,
		block.Reward,
		string(transactionData),
		string(usefulWorkData),
		string(humanityData),
		string(claimData),
		string(authorizationData),
		string(grantData),
		string(revocationData),
		string(authorityChangeData),
		string(authorityProposalData),
		string(authorityExecutionData),
		string(transferProposalData),
		string(transferExecutionData),
	)

	hash := sha256.Sum256(
		[]byte(payload),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

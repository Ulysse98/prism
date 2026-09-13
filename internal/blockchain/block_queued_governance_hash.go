package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// calculateQueuedGovernanceBlockHash calculates the canonical block hash
// for blocks containing queued authority governance objects.
//
// The v0.29 domain commits:
//   - all existing consensus payloads;
//   - legacy direct AuthorityChanges;
//   - AuthorityProposals;
//   - AuthorityExecutions.
//
// This prevents any queued-governance field from being modified without
// changing the block hash.
func calculateQueuedGovernanceBlockHash(
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

	proposalData, err := json.Marshal(
		block.AuthorityProposals,
	)
	if err != nil {
		panic(err)
	}

	executionData, err := json.Marshal(
		block.AuthorityExecutions,
	)
	if err != nil {
		panic(err)
	}

	payload := fmt.Sprintf(
		"queued-governance-block-v1|%d|%s|%s|%s|%d|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
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
		string(proposalData),
		string(executionData),
	)

	hash := sha256.Sum256(
		[]byte(payload),
	)

	return hex.EncodeToString(
		hash[:],
	)
}

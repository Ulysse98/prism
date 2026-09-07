package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"prism/internal/consensus"
	"prism/internal/identity"
	"prism/internal/poup"
	"prism/internal/reserved"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
)

// Compatibility aliases for code written before v0.20.
// Consensus owns the canonical reward parameters.
const BlockReward uint64 = consensus.DefaultProposerReward
const UsefulWorkReward uint64 = consensus.DefaultUsefulWorkReward

type Block struct {
	Height                 uint64                    `json:"height"`
	Timestamp              time.Time                 `json:"timestamp"`
	PreviousHash           string                    `json:"previous_hash"`
	Proposer               string                    `json:"proposer"`
	Reward                 uint64                    `json:"reward"`
	Transactions           []transaction.Transaction `json:"transactions"`
	UsefulWork             []usefulwork.Proof        `json:"useful_work"`
	Humanity               []identity.Attestation    `json:"humanity,omitempty"`
	ParticipationClaims    []poup.Claim              `json:"participation_claims,omitempty"`
	ReservedAuthorizations []reserved.Authorization  `json:"reserved_authorizations,omitempty"`
	Hash                   string                    `json:"hash"`
}

func CalculateHash(
	block Block,
) string {

	transactionData, err := json.Marshal(
		block.Transactions,
	)
	if err != nil {
		panic(err)
	}

	usefulWorkData, err := json.Marshal(
		block.UsefulWork,
	)
	if err != nil {
		panic(err)
	}

	// Reserved-authorization-aware hash format.
	//
	// Existing blocks without reserved authorizations retain
	// their exact legacy, humanity or PoUP hash formats below.
	if len(block.ReservedAuthorizations) > 0 {
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

		reservedData, err := json.Marshal(
			block.ReservedAuthorizations,
		)
		if err != nil {
			panic(err)
		}

		payload := fmt.Sprintf(
			"reserved-block-v1|%d|%s|%s|%s|%d|%s|%s|%s|%s|%s",
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
			string(reservedData),
		)

		hash := sha256.Sum256(
			[]byte(payload),
		)

		return hex.EncodeToString(hash[:])
	}

	// PoUP claim-aware hash format.
	//
	// Existing blocks without participation claims continue
	// through the legacy or humanity-aware formats below.
	if len(block.ParticipationClaims) > 0 {
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

		payload := fmt.Sprintf(
			"poup-v1|%d|%s|%s|%s|%d|%s|%s|%s|%s",
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
		)

		hash := sha256.Sum256(
			[]byte(payload),
		)

		return hex.EncodeToString(hash[:])
	}

	// Legacy hash format.
	//
	// Blocks created before humanity attestations existed must
	// retain exactly the same hash so existing Prism chains
	// continue to validate.
	if len(block.Humanity) == 0 {
		payload := fmt.Sprintf(
			"%d|%s|%s|%s|%d|%s|%s",
			block.Height,
			block.Timestamp.UTC().Format(
				time.RFC3339Nano,
			),
			block.PreviousHash,
			block.Proposer,
			block.Reward,
			string(transactionData),
			string(usefulWorkData),
		)

		hash := sha256.Sum256(
			[]byte(payload),
		)

		return hex.EncodeToString(hash[:])
	}

	humanityData, err := json.Marshal(
		block.Humanity,
	)
	if err != nil {
		panic(err)
	}

	// Humanity-aware hash format.
	payload := fmt.Sprintf(
		"%d|%s|%s|%s|%d|%s|%s|%s",
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
	)

	hash := sha256.Sum256(
		[]byte(payload),
	)

	return hex.EncodeToString(hash[:])
}
func CreateGenesisBlock(
	initialBalances map[string]uint64,
) (Block, error) {

	accounts := make(
		[]string,
		0,
		len(initialBalances),
	)

	for account := range initialBalances {
		accounts = append(
			accounts,
			account,
		)
	}

	sort.Strings(accounts)

	transactions := make(
		[]transaction.Transaction,
		0,
	)

	for _, account := range accounts {
		if account == "" {
			return Block{}, fmt.Errorf(
				"genesis account cannot be empty",
			)
		}

		if account == "GENESIS" {
			return Block{}, fmt.Errorf(
				"GENESIS is a reserved account",
			)
		}

		amount := initialBalances[account]

		if amount == 0 {
			continue
		}

		tx := transaction.NewGenesis(
			account,
			amount,
		)

		if err := transaction.ValidateGenesis(
			tx,
		); err != nil {
			return Block{}, err
		}

		transactions = append(
			transactions,
			tx,
		)
	}

	block := Block{
		Height:       0,
		Timestamp:    time.Now().UTC(),
		PreviousHash: "0",
		Proposer:     "GENESIS",
		Reward:       0,
		Transactions: transactions,
		UsefulWork:   []usefulwork.Proof{},
	}

	block.Hash = CalculateHash(block)

	return block, nil
}

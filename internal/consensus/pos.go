package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
)

type Validator struct {
	Address string `json:"address"`
	Stake   uint64 `json:"stake"`
}

type ProofOfStake struct {
	Validators []Validator
}

func NewProofOfStake() *ProofOfStake {
	return &ProofOfStake{
		Validators: make([]Validator, 0),
	}
}

func (pos *ProofOfStake) Register(
	address string,
	stake uint64,
) error {

	if address == "" {
		return fmt.Errorf(
			"validator address cannot be empty",
		)
	}

	if address == "GENESIS" {
		return fmt.Errorf(
			"GENESIS cannot become a validator",
		)
	}

	if stake == 0 {
		return fmt.Errorf(
			"validator stake must be greater than zero",
		)
	}

	for _, validator := range pos.Validators {
		if validator.Address == address {
			return fmt.Errorf(
				"validator already registered",
			)
		}
	}

	if pos.TotalStake() > math.MaxUint64-stake {
		return fmt.Errorf(
			"total stake overflow",
		)
	}

	pos.Validators = append(
		pos.Validators,
		Validator{
			Address: address,
			Stake:   stake,
		},
	)

	return nil
}

func (pos *ProofOfStake) Unregister(
	address string,
) (uint64, error) {

	for index, validator := range pos.Validators {
		if validator.Address != address {
			continue
		}

		stake := validator.Stake

		pos.Validators = append(
			pos.Validators[:index],
			pos.Validators[index+1:]...,
		)

		return stake, nil
	}

	return 0, fmt.Errorf(
		"validator not registered",
	)
}

func (pos *ProofOfStake) StakeOf(
	address string,
) uint64 {

	for _, validator := range pos.Validators {
		if validator.Address == address {
			return validator.Stake
		}
	}

	return 0
}

func (pos *ProofOfStake) IsValidator(
	address string,
) bool {

	return pos.StakeOf(address) > 0
}

func (pos *ProofOfStake) TotalStake() uint64 {
	var total uint64

	for _, validator := range pos.Validators {
		total += validator.Stake
	}

	return total
}

func (pos *ProofOfStake) SelectProposer(
	previousHash string,
	nextHeight uint64,
) (Validator, error) {

	totalStake := pos.TotalStake()

	if totalStake == 0 {
		return Validator{}, fmt.Errorf(
			"no active stake available",
		)
	}

	payload := fmt.Sprintf(
		"%s|%d",
		previousHash,
		nextHeight,
	)

	hash := sha256.Sum256(
		[]byte(payload),
	)

	randomValue := binary.BigEndian.Uint64(
		hash[:8],
	)

	ticket := randomValue % totalStake

	var cumulative uint64

	for _, validator := range pos.Validators {
		cumulative += validator.Stake

		if ticket < cumulative {
			return validator, nil
		}
	}

	return Validator{}, fmt.Errorf(
		"unable to select proposer",
	)
}

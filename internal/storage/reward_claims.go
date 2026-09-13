package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/participation"
)

const rewardClaimsFilename = "reward_claims.json"

type RewardClaimRecord struct {
	Address string `json:"address"`
	Period  uint64 `json:"period"`
}

type RewardClaimsFile struct {
	Claims []RewardClaimRecord `json:"claims"`
}

func SaveRewardClaims(
	dataDir string,
	book *participation.RewardClaimBook,
) error {
	if book == nil {
		return fmt.Errorf(
			"reward claim book cannot be nil",
		)
	}

	if err := os.MkdirAll(
		dataDir,
		0755,
	); err != nil {
		return err
	}

	claims := book.Claims()

	records := make(
		[]RewardClaimRecord,
		0,
		len(claims),
	)

	for _, claim := range claims {
		records = append(
			records,
			RewardClaimRecord{
				Address: claim.Address,
				Period:  claim.Period,
			},
		)
	}

	state := RewardClaimsFile{
		Claims: records,
	}

	data, err := json.MarshalIndent(
		state,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	path := filepath.Join(
		dataDir,
		rewardClaimsFilename,
	)

	return os.WriteFile(
		path,
		data,
		0600,
	)
}

func LoadRewardClaims(
	dataDir string,
) (*participation.RewardClaimBook, error) {
	path := filepath.Join(
		dataDir,
		rewardClaimsFilename,
	)

	data, err := os.ReadFile(path)

	if os.IsNotExist(err) {
		return participation.NewRewardClaimBook(), nil
	}

	if err != nil {
		return nil, err
	}

	var state RewardClaimsFile

	if err := json.Unmarshal(
		data,
		&state,
	); err != nil {
		return nil, err
	}

	claims := make(
		[]participation.RewardClaimKey,
		0,
		len(state.Claims),
	)

	for _, record := range state.Claims {
		claims = append(
			claims,
			participation.RewardClaimKey{
				Address: record.Address,
				Period:  record.Period,
			},
		)
	}

	book, err :=
		participation.NewRewardClaimBookFromClaims(
			claims,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"stored reward claims are invalid: %w",
			err,
		)
	}

	return book, nil
}

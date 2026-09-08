package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"prism/internal/reserved"
)

type ChainConfig struct {
	ReservedAuthorities reserved.AuthorityPolicy `json:"reserved_authorities"`
}

func DefaultChainConfig() ChainConfig {
	return ChainConfig{
		ReservedAuthorities: reserved.DefaultAuthorityPolicy(),
	}
}

func (config ChainConfig) IsLegacy() bool {
	policy :=
		config.ReservedAuthorities

	return len(policy.Ecosystem) == 0 &&
		len(policy.Treasury) == 0 &&
		len(policy.Team) == 0 &&
		len(policy.Liquidity) == 0
}

func canonicalAuthorityList(
	poolName string,
	authorities []string,
) ([]string, error) {
	if len(authorities) == 0 {
		return nil, nil
	}

	canonical :=
		append(
			[]string(nil),
			authorities...,
		)

	sort.Strings(
		canonical,
	)

	for index, authority := range canonical {

		if authority == "" {
			return nil,
				fmt.Errorf(
					"%s reserved authority cannot be empty",
					poolName,
				)
		}

		if authority == "GENESIS" {
			return nil,
				fmt.Errorf(
					"GENESIS cannot be a %s reserved authority",
					poolName,
				)
		}

		if index > 0 &&
			authority ==
				canonical[index-1] {

			return nil,
				fmt.Errorf(
					"duplicate %s reserved authority: %s",
					poolName,
					authority,
				)
		}
	}

	return canonical, nil
}

func (config ChainConfig) Canonical() (
	ChainConfig,
	error,
) {
	ecosystem, err :=
		canonicalAuthorityList(
			"ecosystem",
			config.ReservedAuthorities.Ecosystem,
		)

	if err != nil {
		return ChainConfig{}, err
	}

	treasury, err :=
		canonicalAuthorityList(
			"treasury",
			config.ReservedAuthorities.Treasury,
		)

	if err != nil {
		return ChainConfig{}, err
	}

	team, err :=
		canonicalAuthorityList(
			"team",
			config.ReservedAuthorities.Team,
		)

	if err != nil {
		return ChainConfig{}, err
	}

	liquidity, err :=
		canonicalAuthorityList(
			"liquidity",
			config.ReservedAuthorities.Liquidity,
		)

	if err != nil {
		return ChainConfig{}, err
	}

	policy := reserved.AuthorityPolicy{
		Ecosystem: ecosystem,
		Treasury:  treasury,
		Team:      team,
		Liquidity: liquidity,

		EcosystemThreshold: config.ReservedAuthorities.EcosystemThreshold,
		TreasuryThreshold:  config.ReservedAuthorities.TreasuryThreshold,
		TeamThreshold:      config.ReservedAuthorities.TeamThreshold,
		LiquidityThreshold: config.ReservedAuthorities.LiquidityThreshold,
	}

	if err := policy.Validate(); err != nil {
		return ChainConfig{}, err
	}

	return ChainConfig{
		ReservedAuthorities: policy,
	}, nil
}

func (config ChainConfig) Commitment() (
	string,
	error,
) {
	canonical, err :=
		config.Canonical()

	if err != nil {
		return "", err
	}

	data, err :=
		json.Marshal(
			canonical,
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"cannot encode chain config: %w",
				err,
			)
	}

	payload :=
		append(
			[]byte(
				"prism-chain-config-v1|",
			),
			data...,
		)

	hash :=
		sha256.Sum256(
			payload,
		)

	return hex.EncodeToString(
		hash[:],
	), nil
}

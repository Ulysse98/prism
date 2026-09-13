package reserved

import (
	"fmt"
	"sort"

	"prism/internal/consensus"
)

type AuthorizationReplayRecord struct {
	Pool       consensus.ReservedPool `json:"pool"`
	Authorizer string                 `json:"authorizer"`
	Nonce      uint64                 `json:"nonce"`
}

type GrantReplayRecord struct {
	Pool  consensus.ReservedPool `json:"pool"`
	Nonce uint64                 `json:"nonce"`
}

type RevokedGrantRecord struct {
	Pool    consensus.ReservedPool `json:"pool"`
	GrantID string                 `json:"grant_id"`
}

type AuthorityChangeReplayRecord struct {
	Pool  consensus.ReservedPool `json:"pool"`
	Nonce uint64                 `json:"nonce"`
}

type ReplaySnapshot struct {
	UsedAuthorizationIDs []string `json:"used_authorization_ids"`

	AuthorizationNonces []AuthorizationReplayRecord `json:"authorization_nonces"`

	UsedGrantIDs []string `json:"used_grant_ids"`

	GrantNonces []GrantReplayRecord `json:"grant_nonces"`

	RevokedGrants []RevokedGrantRecord `json:"revoked_grants"`

	UsedAuthorityChangeIDs []string `json:"used_authority_change_ids"`

	AuthorityChangeNonces []AuthorityChangeReplayRecord `json:"authority_change_nonces"`
}

func (state *ReplayState) Snapshot() (
	ReplaySnapshot,
	error,
) {
	if state == nil {
		return ReplaySnapshot{},
			fmt.Errorf(
				"reserved replay state cannot be nil",
			)
	}

	snapshot := ReplaySnapshot{}

	for id := range state.usedIDs {
		snapshot.UsedAuthorizationIDs =
			append(
				snapshot.UsedAuthorizationIDs,
				id,
			)
	}

	for key, nonce := range state.lastNonce {
		snapshot.AuthorizationNonces =
			append(
				snapshot.AuthorizationNonces,
				AuthorizationReplayRecord{
					Pool:       key.Pool,
					Authorizer: key.Authorizer,
					Nonce:      nonce,
				},
			)
	}

	for id := range state.usedGrantIDs {
		snapshot.UsedGrantIDs =
			append(
				snapshot.UsedGrantIDs,
				id,
			)
	}

	for key, nonce := range state.lastGrantNonce {
		snapshot.GrantNonces =
			append(
				snapshot.GrantNonces,
				GrantReplayRecord{
					Pool:  key.Pool,
					Nonce: nonce,
				},
			)
	}

	for key := range state.revokedGrants {
		snapshot.RevokedGrants =
			append(
				snapshot.RevokedGrants,
				RevokedGrantRecord{
					Pool:    key.Pool,
					GrantID: key.GrantID,
				},
			)
	}

	for id := range state.usedAuthorityChangeIDs {
		snapshot.UsedAuthorityChangeIDs =
			append(
				snapshot.UsedAuthorityChangeIDs,
				id,
			)
	}

	for key, nonce := range state.lastAuthorityChangeNonce {

		snapshot.AuthorityChangeNonces =
			append(
				snapshot.AuthorityChangeNonces,
				AuthorityChangeReplayRecord{
					Pool:  key.Pool,
					Nonce: nonce,
				},
			)
	}

	sort.Strings(snapshot.UsedAuthorizationIDs)
	sort.Strings(snapshot.UsedGrantIDs)
	sort.Strings(snapshot.UsedAuthorityChangeIDs)

	sort.Slice(
		snapshot.AuthorizationNonces,
		func(i, j int) bool {
			left := snapshot.AuthorizationNonces[i]
			right := snapshot.AuthorizationNonces[j]

			if left.Pool != right.Pool {
				return left.Pool < right.Pool
			}

			return left.Authorizer < right.Authorizer
		},
	)

	sort.Slice(
		snapshot.GrantNonces,
		func(i, j int) bool {
			return snapshot.GrantNonces[i].Pool <
				snapshot.GrantNonces[j].Pool
		},
	)

	sort.Slice(
		snapshot.RevokedGrants,
		func(i, j int) bool {
			left := snapshot.RevokedGrants[i]
			right := snapshot.RevokedGrants[j]

			if left.Pool != right.Pool {
				return left.Pool < right.Pool
			}

			return left.GrantID < right.GrantID
		},
	)

	sort.Slice(
		snapshot.AuthorityChangeNonces,
		func(i, j int) bool {
			return snapshot.AuthorityChangeNonces[i].Pool <
				snapshot.AuthorityChangeNonces[j].Pool
		},
	)

	return snapshot, nil
}

func ReplayStateFromSnapshot(
	snapshot ReplaySnapshot,
) (
	*ReplayState,
	error,
) {
	state := NewReplayState()

	for _, id := range snapshot.UsedAuthorizationIDs {
		if id == "" {
			return nil,
				fmt.Errorf(
					"stored authorization ID cannot be empty",
				)
		}

		if _, exists := state.usedIDs[id]; exists {
			return nil,
				fmt.Errorf(
					"duplicate stored authorization ID",
				)
		}

		state.usedIDs[id] = struct{}{}
	}

	for _, record := range snapshot.AuthorizationNonces {

		if err := validateReplayPool(
			record.Pool,
		); err != nil {
			return nil, err
		}

		if record.Authorizer == "" {
			return nil,
				fmt.Errorf(
					"stored authorization authorizer cannot be empty",
				)
		}

		if record.Nonce == 0 {
			return nil,
				fmt.Errorf(
					"stored authorization nonce must be greater than zero",
				)
		}

		key := replayKey{
			Pool:       record.Pool,
			Authorizer: record.Authorizer,
		}

		if _, exists := state.lastNonce[key]; exists {
			return nil,
				fmt.Errorf(
					"duplicate stored authorization nonce entry",
				)
		}

		state.lastNonce[key] = record.Nonce
	}

	for _, id := range snapshot.UsedGrantIDs {
		if id == "" {
			return nil,
				fmt.Errorf(
					"stored grant ID cannot be empty",
				)
		}

		if _, exists :=
			state.usedGrantIDs[id]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored grant ID",
				)
		}

		state.usedGrantIDs[id] = struct{}{}
	}

	for _, record := range snapshot.GrantNonces {
		if err := validateReplayPool(
			record.Pool,
		); err != nil {
			return nil, err
		}

		if record.Nonce == 0 {
			return nil,
				fmt.Errorf(
					"stored grant nonce must be greater than zero",
				)
		}

		key := grantReplayKey{
			Pool: record.Pool,
		}

		if _, exists :=
			state.lastGrantNonce[key]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored grant nonce entry",
				)
		}

		state.lastGrantNonce[key] = record.Nonce
	}

	for _, record := range snapshot.RevokedGrants {
		if err := validateReplayPool(
			record.Pool,
		); err != nil {
			return nil, err
		}

		if record.GrantID == "" {
			return nil,
				fmt.Errorf(
					"stored revoked grant ID cannot be empty",
				)
		}

		key := revocationReplayKey{
			Pool:    record.Pool,
			GrantID: record.GrantID,
		}

		if _, exists :=
			state.revokedGrants[key]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored revoked grant",
				)
		}

		state.revokedGrants[key] = struct{}{}
	}

	for _, id := range snapshot.UsedAuthorityChangeIDs {

		if id == "" {
			return nil,
				fmt.Errorf(
					"stored authority change ID cannot be empty",
				)
		}

		if _, exists :=
			state.usedAuthorityChangeIDs[id]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored authority change ID",
				)
		}

		state.usedAuthorityChangeIDs[id] =
			struct{}{}
	}

	for _, record := range snapshot.AuthorityChangeNonces {

		if err := validateReplayPool(
			record.Pool,
		); err != nil {
			return nil, err
		}

		if record.Nonce == 0 {
			return nil,
				fmt.Errorf(
					"stored authority change nonce must be greater than zero",
				)
		}

		key := authorityChangeReplayKey{
			Pool: record.Pool,
		}

		if _, exists :=
			state.lastAuthorityChangeNonce[key]; exists {

			return nil,
				fmt.Errorf(
					"duplicate stored authority change nonce entry",
				)
		}

		state.lastAuthorityChangeNonce[key] =
			record.Nonce
	}

	return state, nil
}

func validateReplayPool(
	pool consensus.ReservedPool,
) error {
	switch pool {
	case consensus.ReservedPoolEcosystem,
		consensus.ReservedPoolTreasury,
		consensus.ReservedPoolTeam,
		consensus.ReservedPoolLiquidity:
		return nil

	default:
		return fmt.Errorf(
			"invalid stored reserved pool: %q",
			pool,
		)
	}
}

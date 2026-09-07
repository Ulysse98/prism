package reserved

import (
	"fmt"

	"prism/internal/consensus"
)

type replayKey struct {
	Pool       consensus.ReservedPool
	Authorizer string
}

type ReplayState struct {
	usedIDs   map[string]struct{}
	lastNonce map[replayKey]uint64
}

func NewReplayState() *ReplayState {
	return &ReplayState{
		usedIDs:   make(map[string]struct{}),
		lastNonce: make(map[replayKey]uint64),
	}
}

func (state *ReplayState) ValidateNext(
	authorization Authorization,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if authorization.ID == "" {
		return fmt.Errorf(
			"reserved authorization ID cannot be empty",
		)
	}

	if authorization.Nonce == 0 {
		return fmt.Errorf(
			"reserved authorization nonce must be greater than zero",
		)
	}

	if _, exists :=
		state.usedIDs[authorization.ID]; exists {

		return fmt.Errorf(
			"reserved authorization already used",
		)
	}

	key := replayKey{
		Pool:       authorization.Pool,
		Authorizer: authorization.Authorizer,
	}

	if last, exists :=
		state.lastNonce[key]; exists &&
		authorization.Nonce <= last {

		return fmt.Errorf(
			"reserved authorization nonce is not increasing: nonce=%d last=%d",
			authorization.Nonce,
			last,
		)
	}

	return nil
}

func (state *ReplayState) Accept(
	authorization Authorization,
	policy AuthorityPolicy,
	expectedChainID string,
) error {
	if state == nil {
		return fmt.Errorf(
			"reserved replay state cannot be nil",
		)
	}

	if err :=
		policy.ValidateAuthorization(
			authorization,
			expectedChainID,
		); err != nil {

		return err
	}

	if err :=
		state.ValidateNext(
			authorization,
		); err != nil {

		return err
	}

	if state.usedIDs == nil {
		state.usedIDs =
			make(map[string]struct{})
	}

	if state.lastNonce == nil {
		state.lastNonce =
			make(map[replayKey]uint64)
	}

	key := replayKey{
		Pool:       authorization.Pool,
		Authorizer: authorization.Authorizer,
	}

	state.usedIDs[authorization.ID] = struct{}{}

	state.lastNonce[key] =
		authorization.Nonce

	return nil
}

//go:build !linux

package governancev069

import "errors"

var ErrAdvisoryVoteAlreadyReserved = errors.New("advisory vote already reserved for voter and proposal")

type AdvisoryVoteReplayStore struct{}

func OpenAdvisoryVoteReplayStore(string) (*AdvisoryVoteReplayStore, error) {
	return nil, errors.New("v0.69 local durable advisory replay lab is Linux-only")
}
func (*AdvisoryVoteReplayStore) ReserveVerifiedAdvisoryVote(SignedAdvisoryVote, ProposalIntent, Context) ([32]byte, error) {
	return [32]byte{}, errors.New("v0.69 local durable advisory replay lab is Linux-only")
}

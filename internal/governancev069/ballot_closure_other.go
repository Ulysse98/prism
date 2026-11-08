//go:build !linux

package governancev069

import "errors"

func SealLocalAdvisoryVoteStore(*AdvisoryVoteReplayStore, AdvisoryTallyReceipt, EligibilitySnapshot, string, string, ProposalIntent, Context, []SignedAdvisoryVote) (AdvisoryBallotClosure, error) {
	return AdvisoryBallotClosure{}, errors.New("v0.69 local advisory ballot closure requires Linux")
}
func VerifySealedLocalAdvisoryVoteStore(*AdvisoryVoteReplayStore, AdvisoryTallyReceipt, EligibilitySnapshot, string, string, ProposalIntent, Context, []SignedAdvisoryVote) (AdvisoryBallotClosure, error) {
	return AdvisoryBallotClosure{}, errors.New("v0.69 local advisory ballot closure requires Linux")
}

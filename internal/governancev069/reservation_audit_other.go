//go:build !linux

package governancev069

import "errors"

func AuditLocalAdvisoryReservations(*AdvisoryVoteReplayStore, AdvisoryTallyReceipt, EligibilitySnapshot, string, string, ProposalIntent, Context, []SignedAdvisoryVote) (AdvisoryReservationAudit, error) {
	return AdvisoryReservationAudit{}, errors.New("v0.69 reservation reconciliation audit is Linux-only")
}
func VerifyLocalAdvisoryReservationAudit(AdvisoryReservationAudit, *AdvisoryVoteReplayStore, AdvisoryTallyReceipt, EligibilitySnapshot, string, string, ProposalIntent, Context, []SignedAdvisoryVote) error {
	return errors.New("v0.69 reservation reconciliation audit is Linux-only")
}

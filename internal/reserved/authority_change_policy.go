package reserved

import "fmt"

// ValidateAuthorityChange validates an authority-set change against
// the currently active authority policy.
//
// Governance rule:
// the current authority set for the target pool authorizes the change.
// A proposed authority does not gain voting power until the change has
// actually been applied.
func (policy AuthorityPolicy) ValidateAuthorityChange(
	change AuthorityChange,
	expectedChainID string,
) error {
	if err :=
		ValidateAuthorityChange(change); err != nil {

		return err
	}

	if expectedChainID == "" {
		return fmt.Errorf(
			"expected chain ID cannot be empty",
		)
	}

	if change.ChainID != expectedChainID {
		return fmt.Errorf(
			"reserved authority change chain ID mismatch",
		)
	}

	if err := policy.Validate(); err != nil {
		return err
	}

	authorities, err :=
		policy.authorities(change.Pool)

	if err != nil {
		return err
	}

	threshold, err :=
		policy.EffectiveThreshold(
			change.Pool,
		)

	if err != nil {
		return err
	}

	allowed :=
		make(
			map[string]struct{},
			len(authorities),
		)

	for _, authority := range authorities {

		allowed[authority] =
			struct{}{}
	}

	approved :=
		make(
			map[string]struct{},
		)

	for _, approval := range change.Approvals {

		if err :=
			ValidateAuthorityChangeApproval(
				change,
				approval,
			); err != nil {

			return err
		}

		if _, exists :=
			allowed[approval.Authorizer]; !exists {

			return fmt.Errorf(
				"authority change approver is not authorized for reserved pool",
			)
		}

		if _, exists :=
			approved[approval.Authorizer]; exists {

			return fmt.Errorf(
				"duplicate reserved authority change approver",
			)
		}

		approved[approval.Authorizer] =
			struct{}{}
	}

	if uint32(len(approved)) <
		threshold {

		return fmt.Errorf(
			"reserved authority change approval threshold not met: approvals=%d threshold=%d",
			len(approved),
			threshold,
		)
	}

	return nil
}

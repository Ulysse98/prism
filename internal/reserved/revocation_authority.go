package reserved

import "fmt"

func (policy AuthorityPolicy) ValidateRevocation(
	revocation Revocation,
	expectedChainID string,
) error {
	if err :=
		ValidateRevocation(revocation); err != nil {

		return err
	}

	if expectedChainID == "" {
		return fmt.Errorf(
			"expected chain ID cannot be empty",
		)
	}

	if revocation.ChainID != expectedChainID {
		return fmt.Errorf(
			"reserved revocation chain ID mismatch",
		)
	}

	if err := policy.Validate(); err != nil {
		return err
	}

	authorities, err :=
		policy.authorities(
			revocation.Pool,
		)

	if err != nil {
		return err
	}

	threshold, err :=
		policy.EffectiveThreshold(
			revocation.Pool,
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

	for _, approval := range revocation.Approvals {

		if err :=
			ValidateRevocationApproval(
				revocation,
				approval,
			); err != nil {

			return err
		}

		if _, exists :=
			allowed[approval.Authorizer]; !exists {

			return fmt.Errorf(
				"revocation approver is not authorized for reserved pool",
			)
		}

		if _, exists :=
			approved[approval.Authorizer]; exists {

			return fmt.Errorf(
				"duplicate reserved revocation approver",
			)
		}

		approved[approval.Authorizer] =
			struct{}{}
	}

	if uint32(len(approved)) < threshold {
		return fmt.Errorf(
			"reserved revocation approval threshold not met: approvals=%d threshold=%d",
			len(approved),
			threshold,
		)
	}

	return nil
}

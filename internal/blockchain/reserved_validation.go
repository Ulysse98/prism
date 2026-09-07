package blockchain

import (
	"fmt"

	"prism/internal/reserved"
)

func (bc *Blockchain) ValidateReservedAuthorization(
	authorization reserved.Authorization,
	authorityPolicy reserved.AuthorityPolicy,
) error {
	chainID, err :=
		bc.ChainID()

	if err != nil {
		return fmt.Errorf(
			"cannot determine chain identity: %w",
			err,
		)
	}

	if err :=
		authorityPolicy.ValidateAuthorization(
			authorization,
			chainID,
		); err != nil {

		return fmt.Errorf(
			"invalid reserved authorization: %w",
			err,
		)
	}

	return nil
}

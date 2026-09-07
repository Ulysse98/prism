package blockchain

import "fmt"

func (bc *Blockchain) ReservedEmission() (
	uint64,
	error,
) {
	accounting, err :=
		bc.GetReservedAccountingState()

	if err != nil {
		return 0,
			fmt.Errorf(
				"cannot calculate reserved accounting: %w",
				err,
			)
	}

	emission, err :=
		accounting.Usage.ExplicitTotal()

	if err != nil {
		return 0,
			fmt.Errorf(
				"cannot calculate reserved emission: %w",
				err,
			)
	}

	return emission, nil
}

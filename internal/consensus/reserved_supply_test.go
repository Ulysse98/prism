package consensus

import "testing"

func TestReservedPoolAllocations(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	tests := []struct {
		pool     ReservedPool
		expected uint64
	}{
		{
			pool:     ReservedPoolEcosystem,
			expected: 15_000_000,
		},
		{
			pool:     ReservedPoolTreasury,
			expected: 10_000_000,
		},
		{
			pool:     ReservedPoolTeam,
			expected: 10_000_000,
		},
		{
			pool:     ReservedPoolLiquidity,
			expected: 5_000_000,
		},
	}

	for _, test := range tests {
		allocation, err :=
			policy.ReservedPoolAllocation(
				test.pool,
			)

		if err != nil {
			t.Fatal(err)
		}

		if allocation != test.expected {
			t.Fatalf(
				"pool %s: expected %d, got %d",
				test.pool,
				test.expected,
				allocation,
			)
		}
	}
}

func TestReservedPoolAllocationRejectsUnknownPool(
	t *testing.T,
) {
	policy :=
		DefaultSupplyPolicy()

	if _, err :=
		policy.ReservedPoolAllocation(
			ReservedPool("unknown"),
		); err == nil {

		t.Fatal(
			"expected unknown reserved pool to fail",
		)
	}
}

package crosschain

import "testing"

func TestNewReceiptMatchesCrossChainVector(
	t *testing.T,
) {
	t.Parallel()

	receipt, err := NewReceipt(
		"dbab0c89f752689748f4b1d375cd94800b40a1ed299e028edfa2336c0ba9a6cd",
		"42b5df2e467b316cd4fe45bdc303c15494854b917d8b98ee82402d1ef9076c38",
		"prism-worker-alice",
		"prism-d8c1f3e740b48957",
	)
	if err != nil {
		t.Fatalf(
			"NewReceipt() error = %v",
			err,
		)
	}

	const expectedWorkerHash = "0x9ef8a0bd338cccf1544c7c24b01e5149c939e254fa9c22bfc0c9a6fde7496726"

	const expectedChainHash = "0x27a955f0f028e1b3f397d20c36a77c7b8546fe31dd754bcf1fbdaae3ad95aedb"

	const expectedRegistryID = "0x0d2f7411a6f9e0263209bcc7278d172346e0b5916902bb22efc8c0ffbb1a618e"

	if receipt.WorkerIDHash != expectedWorkerHash {
		t.Fatalf(
			"worker hash = %s, want %s",
			receipt.WorkerIDHash,
			expectedWorkerHash,
		)
	}

	if receipt.PrismChainIDHash != expectedChainHash {
		t.Fatalf(
			"chain hash = %s, want %s",
			receipt.PrismChainIDHash,
			expectedChainHash,
		)
	}

	if receipt.RegistryID != expectedRegistryID {
		t.Fatalf(
			"registry ID = %s, want %s",
			receipt.RegistryID,
			expectedRegistryID,
		)
	}
}

func TestNewReceiptRejectsInvalidIDs(
	t *testing.T,
) {
	t.Parallel()

	_, err := NewReceipt(
		"not-a-job-id",
		"42b5df2e467b316cd4fe45bdc303c15494854b917d8b98ee82402d1ef9076c38",
		"worker",
		"prism-chain",
	)

	if err == nil {
		t.Fatal(
			"expected invalid job ID error",
		)
	}
}

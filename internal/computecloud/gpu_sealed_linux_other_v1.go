//go:build linux && !amd64

package computecloud

// No sealed memfd launcher is approved for other Linux architectures.
// Fail closed rather than silently falling back to pathname execution.
import (
	"errors"
	"os"
)

const cloudMaxWorkerBytesV1 int64 = 256 << 20

func newSealedCloudGPUWorkerV1(string, string, int64) (*os.File, error) {
	return nil, errors.New("Cloud sealed GPU execution requires Linux amd64")
}

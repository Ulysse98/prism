//go:build linux

package crosschain

// NewLocalOperatorAdmitterFromPinnedSynchronizedSourceV067 is the only
// exported v0.67 operator-admission constructor. The privileged, trusted
// caller MUST provide a separately provisioned public-key file path and
// independently trusted SHA-256 fingerprint, plus a cooperative owned source.
// Neither root path nor fingerprint nor source is accepted from an applicant.
// This local entry point does not prove provisioning provenance, network
// finality, global anti-replay, or authority to settle assets.
func NewLocalOperatorAdmitterFromPinnedSynchronizedSourceV067(
	store *LocalReplayStoreV067,
	rootPath, independentlyPinnedSHA256 string,
	signed SignedOperatorTrustV067,
	source *LocalSynchronizedOperatorSourceV067,
) (*LocalOperatorAdmitterV067, error) {
	if source == nil || store == nil {
		return nil, ErrOperatorTrustV067
	}
	key, err := LoadPinnedOperatorRootV067(rootPath, independentlyPinnedSHA256)
	if err != nil {
		return nil, err
	}
	return newLocalOperatorAdmitterFromSynchronizedSourceV067(store, key, signed, source)
}

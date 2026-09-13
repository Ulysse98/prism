package blockchain

// QueuedGovernanceActivationHeight is the first block height at which
// Prism v0.29 queued authority governance is mandatory.
//
// Historical blocks below this height retain the v0.27/v0.28 direct
// AuthorityChange rules.
//
// At and after this height:
//   - direct AuthorityChanges are forbidden;
//   - AuthorityProposals and AuthorityExecutions are enabled.
const QueuedGovernanceActivationHeight uint64 = 3

// GovernedReservedTransferActivationHeight is the first block height at
// which Prism v0.30 governed reserved transfers are enabled.
//
// Keeping this distinct from QueuedGovernanceActivationHeight preserves
// the v0.29 consensus boundary exactly.
//
// At and after this height:
//   - ReservedTransferProposals and ReservedTransferExecutions are enabled;
//   - legacy ReservedGrants are no longer valid consensus spending.
const GovernedReservedTransferActivationHeight uint64 = 4

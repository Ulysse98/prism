package blockchain

// QueuedGovernanceActivationHeight is the first block height at which
// Prism v0.29 queued governance is mandatory.
//
// Historical blocks below this height retain the v0.27/v0.28 direct
// AuthorityChange rules.
//
// Starting at this height:
//   - direct AuthorityChanges are forbidden;
//   - AuthorityProposals and AuthorityExecutions are enabled.
const QueuedGovernanceActivationHeight uint64 = 3

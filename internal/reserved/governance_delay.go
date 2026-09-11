package reserved

// DefaultGovernanceDelayBlocks is the consensus-enforced minimum delay
// between inclusion of an AuthorityProposal and its execution.
//
// v0.29 keeps this value protocol-defined rather than putting it into
// ChainConfig. This preserves existing chain-config commitments while
// queued governance is introduced.
const DefaultGovernanceDelayBlocks uint64 = 5

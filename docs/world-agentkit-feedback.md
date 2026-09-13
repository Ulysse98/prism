# World AgentKit Feedback — Prism / ETHOnline 2026

## Integration

Prism integrates `@worldcoin/agentkit` as an identity gate for Proof of Useful Participation (PoUP).

The current flow:

1. An agent wallet receives an AgentKit authorization challenge.
2. The wallet signs the AgentKit/SIWE message.
3. Prism validates the AgentKit payload.
4. Prism verifies the wallet signature cryptographically.
5. Prism resolves the verified wallet through World AgentBook.
6. Only a wallet resolved to a World ID-backed human may proceed to PoUP eligibility.

The raw anonymous AgentBook human identifier is not intended to be stored by Prism. A domain-separated SHA-256 reference is generated before it would be passed into the Prism identity layer.

## What worked well

- `createAgentkitClient()` was straightforward to integrate.
- AgentKit message generation and signing worked locally.
- `validateAgentkitMessage()` correctly validated the generated authorization.
- `verifyAgentkitSignature()` successfully recovered the expected agent wallet.
- `createAgentBookVerifier().lookupHuman()` successfully queried the canonical AgentBook deployment on World Chain.
- The SDK made it possible to cleanly separate cryptographic agent authentication from human-backing verification.

Observed successful authentication:

- AgentKit message: VALID
- Agent signature: VERIFIED
- Recovered agent matched the configured wallet.

## AgentBook registration issue

Agent wallet:

`0x0dd39175A33c11e8c712387849a851522E79EF4A`

AgentBook status:

- Network: World Chain (`eip155:480`)
- Status: unregistered
- Contract: `0xA23aB2712eA7BBa896930544C7d6636a96b944dA`

Using:

`npx @worldcoin/agentkit-cli@0.2.0 register <agent-address>`

The CLI successfully:

- queried AgentBook,
- returned nonce `0`,
- generated a World ID verification QR/deep link,
- entered the "Waiting for verification..." state.

The World ID flow then failed with:

`Error (VERIFICATION_FAILED): malformed_request`

## Developer experience feedback

The main difficulty was diagnosing the `malformed_request` failure.

AgentKit CLI 0.2.0 does not expose the World ID app ID, action, or other registration request configuration as CLI options. Because the request is created internally by the CLI, it was difficult to determine which request field World ID considered malformed.

More detailed diagnostics would help, for example:

- the World ID error payload returned to the CLI,
- the rejected request field,
- whether the failure came from app configuration, action configuration, signal format, or environment,
- a debug/verbose mode specifically for the World ID bridge request.

## Failure handling

Prism intentionally fails closed.

Because the agent is currently not registered in AgentBook, the integration returns:

- AgentKit authentication: PASSED
- AgentBook human backing: NOT FOUND
- Prism PoUP access: DENIED

No fake World ID verification or local override is used.

## Sandbox testing

TODO: Repeat the registration and proof flow using the ETHOnline World ID Sandbox environment / Sandbox App when access and configuration are available.

The current production-style registration attempt and its `malformed_request` failure are documented above.

## Suggested improvements

1. Add structured World ID bridge error details to AgentKit CLI.
2. Add a documented debug mode for registration.
3. Clearly document how hackathon Sandbox testing differs from the default AgentBook registration flow.
4. Expose enough request metadata to distinguish client errors from Developer Portal / World App configuration errors.
5. Provide an official minimal end-to-end example showing:
   AgentKit signature -> AgentBook lookup -> application authorization.

## Prism result

The current Prism integration successfully demonstrates the security-critical negative path:

Authenticated agent wallet -> AgentBook lookup -> unregistered agent -> PoUP DENIED.

The positive path will additionally create a Prism humanity attestation once an AgentBook-registered test agent is available.

### Sandbox access request

World ID Sandbox Beta access was requested during ETHOnline 2026.

Status: access request submitted; waiting for Firebase App Distribution invitation before performing the Sandbox proof flow.

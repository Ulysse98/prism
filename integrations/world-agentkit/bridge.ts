import "dotenv/config";

import {
  buildAgentkitSchema,
  createAgentBookVerifier,
  createAgentkitClient,
  parseAgentkitHeader,
  validateAgentkitMessage,
  verifyAgentkitSignature,
} from "@worldcoin/agentkit";

import { privateKeyToAccount } from "viem/accounts";
import { randomUUID, createHash } from "node:crypto";

async function main() {
  const privateKey = process.env.AGENT_PRIVATE_KEY as `0x${string}` | undefined;
  const expectedAddress = process.env.AGENT_ADDRESS;

  if (!privateKey) {
    throw new Error("AGENT_PRIVATE_KEY missing from .env");
  }

  if (!expectedAddress) {
    throw new Error("AGENT_ADDRESS missing from .env");
  }

  const account = privateKeyToAccount(privateKey);

  if (account.address.toLowerCase() !== expectedAddress.toLowerCase()) {
    throw new Error("AGENT_PRIVATE_KEY does not match AGENT_ADDRESS");
  }

  const resource = "https://prism.local/poup";
  const now = new Date();

  const extension = {
    info: {
      domain: "prism.local",
      uri: resource,
      statement: "Authorize a human-backed agent for Prism PoUP",
      version: "1",
      nonce: randomUUID().replaceAll("-", ""),
      issuedAt: now.toISOString(),
      expirationTime: new Date(now.getTime() + 5 * 60_000).toISOString(),
    },
    supportedChains: [
      {
        chainId: "eip155:480",
        type: "eip191" as const,
        signatureScheme: "eip191" as const,
      },
    ],
    schema: buildAgentkitSchema(),
  };

  console.log("========================================");
  console.log("      PRISM × WORLD AGENTKIT BRIDGE");
  console.log("========================================");
  console.log();
  console.log("Agent:", account.address);
  console.log("Network: World Chain");
  console.log();

  // Agent side: sign AgentKit challenge.
  const agentkit = createAgentkitClient({
    signer: {
      address: account.address,
      chainId: "eip155:480",
      type: "eip191",
      signMessage: (message) =>
        account.signMessage({ message }),
    },
  });

  console.log("Creating AgentKit authorization...");
  const header = await agentkit.createHeader(extension);

  // Prism side: parse and validate challenge.
  const payload = parseAgentkitHeader(header);

  const validation = await validateAgentkitMessage(
    payload,
    resource,
  );

  if (!validation.valid) {
    console.log("AgentKit message: INVALID");
    console.log("Reason:", validation.error);
    process.exitCode = 1;
    return;
  }

  console.log("AgentKit message: VALID");

  // Verify wallet ownership / agent signature.
  const signature = await verifyAgentkitSignature(payload);

  if (!signature.valid || !signature.address) {
    console.log("Agent signature: INVALID");
    console.log("Reason:", signature.error);
    process.exitCode = 1;
    return;
  }

  console.log("Agent signature: VERIFIED");
  console.log("Recovered agent:", signature.address);

  // Resolve verified agent through World AgentBook.
  const agentBook = createAgentBookVerifier();
  const humanId = await agentBook.lookupHuman(signature.address);

  console.log();

  if (!humanId) {
    console.log("=== PRISM POUP AUTHORIZATION ===");
    console.log("AgentBook: UNREGISTERED");
    console.log("Human-backed agent: NO");
    console.log("PoUP access: DENIED");
    console.log();
    console.log(
      "Cryptographic agent authentication passed, but AgentBook did not resolve the wallet to a World ID-verified human."
    );

    process.exitCode = 2;
    return;
  }

  const prismHumanHash = createHash("sha256")
    .update(`prism-agentkit:${humanId}`)
    .digest("hex");

  console.log("=== PRISM POUP AUTHORIZATION ===");
  console.log("AgentBook: REGISTERED");
  console.log("Human-backed agent: YES");
  console.log("Human reference: sha256:" + prismHumanHash);
  console.log("PoUP access: ALLOWED");
}

main().catch((error) => {
  console.error();
  console.error("AgentKit bridge failed:");
  console.error(error);
  process.exitCode = 1;
});

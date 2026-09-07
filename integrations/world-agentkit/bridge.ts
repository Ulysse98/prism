import "dotenv/config";
import { createHash } from "node:crypto";
import { createAgentBookVerifier } from "@worldcoin/agentkit";

async function main() {
  const agent = process.env.AGENT_ADDRESS;

  if (!agent) {
    throw new Error("AGENT_ADDRESS missing from .env");
  }

  const agentBook = createAgentBookVerifier();

  console.log("========================================");
  console.log("      PRISM × WORLD AGENTKIT BRIDGE");
  console.log("========================================");
  console.log();
  console.log("Agent:", agent);
  console.log("Network: World Chain");
  console.log("Checking AgentBook...");

  const humanId = await agentBook.lookupHuman(agent);

  if (!humanId) {
    console.log();
    console.log("=== AGENTKIT VERIFICATION ===");
    console.log("AgentBook: UNREGISTERED");
    console.log("Human-backed agent: NO");
    console.log("Prism PoUP eligibility: DENIED");
    console.log();
    console.log(
      "Reason: AgentBook could not resolve this agent to a World ID-verified human."
    );

    process.exitCode = 2;
    return;
  }

  const prismHumanHash = createHash("sha256")
    .update(`prism-agentkit:${humanId}`)
    .digest("hex");

  console.log();
  console.log("=== AGENTKIT VERIFICATION ===");
  console.log("AgentBook: REGISTERED");
  console.log("Human-backed agent: YES");
  console.log("Human reference: sha256:" + prismHumanHash);
  console.log("Prism PoUP eligibility: ALLOWED");
}

main().catch((error) => {
  console.error();
  console.error("AgentKit bridge failed:");
  console.error(error);
  process.exitCode = 1;
});

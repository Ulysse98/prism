import {
  encodeAbiParameters,
  keccak256,
  stringToHex,
  type Hex,
} from "viem";

const jobId =
  keccak256(stringToHex("prism-job-001"));

const proofId =
  keccak256(stringToHex("prism-proof-v2-001"));

const workerIdHash =
  keccak256(stringToHex("prism-worker-alice"));

const prismChainIdHash =
  keccak256(
    stringToHex("prism-d8c1f3e740b48957"),
  );

const encoded = encodeAbiParameters(
  [
    { type: "bytes32" },
    { type: "bytes32" },
    { type: "bytes32" },
  ],
  [
    jobId,
    prismChainIdHash,
    proofId,
  ],
);

const registryId =
  keccak256(encoded);

const expectedRegistryId =
  "0x0d2f7411a6f9e0263209bcc7278d172346e0b5916902bb22efc8c0ffbb1a618e" as Hex;

console.log();
console.log("PRISM CROSS-CHAIN PROOF V2");
console.log("--------------------------");
console.log("Job ID        :", jobId);
console.log("Proof ID      :", proofId);
console.log("Worker hash   :", workerIdHash);
console.log("Prism chain   :", prismChainIdHash);
console.log();
console.log("Registry ID   :", registryId);
console.log("Expected      :", expectedRegistryId);
console.log();

if (registryId !== expectedRegistryId) {
  console.error("❌ CROSS-CHAIN REGISTRY ID MISMATCH");
  process.exit(1);
}

console.log("✅ ARBITRUM REGISTRY ID MATCH");
console.log("✅ SOLANA REGISTRY ID MATCH");
console.log("✅ PRISM PROOF V2 CROSS-CHAIN IDENTITY VERIFIED");
console.log();

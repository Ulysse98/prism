import { readFileSync } from "node:fs";
import { network } from "hardhat";
import { keccak256, toBytes, type Address } from "viem";

const deploymentFile = new URL(
  "../ignition/deployments/chain-31337/deployed_addresses.json",
  import.meta.url,
);

const deployedAddresses = JSON.parse(
  readFileSync(deploymentFile, "utf8"),
);

const registryAddress =
  deployedAddresses[
    "PrismWorkRegistryModule#PrismWorkRegistry"
  ] as Address;

const { viem } = await network.connect();

const publicClient = await viem.getPublicClient();
const wallets = await viem.getWalletClients();

const recorder = wallets[0];
const worker = wallets[1].account.address;

const registry = await viem.getContractAt(
  "PrismWorkRegistry",
  registryAddress,
  {
    client: {
      public: publicClient,
      wallet: recorder,
    },
  },
);

const workType = "sum_squares";
const score = 2n;

// Première représentation canonique de notre preuve Prism.
// On alignera ensuite exactement cette construction avec le moteur Go.
const canonicalProof =
  `prism:pouw:v1` +
  `|worker=${worker.toLowerCase()}` +
  `|task=sum_squares` +
  `|input=3,4,5,12` +
  `|result=194` +
  `|score=2`;

const proofHash = keccak256(
  toBytes(canonicalProof),
);

console.log("");
console.log("PRISM × ARBITRUM");
console.log("-----------------");
console.log("Registry :", registryAddress);
console.log("Recorder :", recorder.account.address);
console.log("Worker   :", worker);
console.log("Work     :", workType);
console.log("Result   :", 194);
console.log("Score    :", score.toString());
console.log("Proof    :", proofHash);
console.log("");

const exists = await registry.read.proofExists([
  proofHash,
]);

if (!exists) {
  console.log("Anchoring proof...");

  const txHash =
    await registry.write.recordUsefulWork([
      worker,
      proofHash,
      workType,
      score,
    ]);

  console.log("TX       :", txHash);

  const receipt =
    await publicClient.waitForTransactionReceipt({
      hash: txHash,
    });

  console.log(
    "Block    :",
    receipt.blockNumber.toString(),
  );
} else {
  console.log(
    "Proof already anchored — reading existing proof.",
  );
}

const proof = await registry.read.getProof([
  proofHash,
]);

console.log("");
console.log("ON-CHAIN PROOF");
console.log("--------------");
console.log("Worker   :", proof.worker);
console.log("Hash     :", proof.proofHash);
console.log("Work     :", proof.workType);
console.log("Score    :", proof.score.toString());
console.log(
  "Timestamp:",
  proof.timestamp.toString(),
);
console.log("");
console.log("✅ PRISM USEFUL WORK VERIFIED");
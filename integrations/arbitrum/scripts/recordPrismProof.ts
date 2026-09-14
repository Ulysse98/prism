import { readFileSync } from "node:fs";
import { network } from "hardhat";
import {
  isHex,
  type Address,
  type Hex,
} from "viem";

function env(name: string): string {
  const value = process.env[name];

  if (!value) {
    throw new Error(
      `Missing environment variable ${name}`,
    );
  }

  return value;
}

const prismWorker = env("PRISM_WORKER");
const workType = env("PRISM_WORK_TYPE");
const proofIdRaw = env("PRISM_PROOF_ID");
const scoreRaw = env("PRISM_SCORE");

if (!/^[0-9a-fA-F]{64}$/.test(proofIdRaw)) {
  throw new Error(
    "Prism proof ID must contain exactly 64 hexadecimal characters",
  );
}

const proofId =
  `0x${proofIdRaw}` as Hex;

if (!isHex(proofId, { strict: true })) {
  throw new Error("Invalid Prism proof ID");
}

const score = BigInt(scoreRaw);

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

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

const recorder = wallets[0];

const registry =
  await viem.getContractAt(
    "PrismWorkRegistry",
    registryAddress,
    {
      client: {
        public: publicClient,
        wallet: recorder,
      },
    },
  );

console.log();
console.log("PRISM × ARBITRUM ANCHOR");
console.log("-----------------------");
console.log("Registry :", registryAddress);
console.log("Worker   :", prismWorker);
console.log("Work     :", workType);
console.log("Score    :", score.toString());
console.log("Proof ID :", proofId);
console.log();

const exists =
  await registry.read.proofExists([
    proofId,
  ]);

if (exists) {
  console.log(
    "Proof already anchored — no transaction sent.",
  );
} else {
  console.log("Anchoring verified Prism proof...");

  const txHash =
    await registry.write.recordUsefulWork([
      prismWorker,
      proofId,
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
}

const proof =
  await registry.read.getProof([
    proofId,
  ]);

console.log();
console.log("ARBITRUM ANCHOR");
console.log("----------------");
console.log("Prism worker :", proof.prismWorker);
console.log("Proof ID     :", proof.proofId);
console.log("Work type    :", proof.workType);
console.log("Score        :", proof.score.toString());
console.log(
  "Anchored at  :",
  proof.anchoredAt.toString(),
);
console.log();
console.log(
  "✅ VERIFIED PRISM PoUW PROOF ANCHORED",
);
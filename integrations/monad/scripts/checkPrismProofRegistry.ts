import { readFileSync } from "node:fs";
import { network } from "hardhat";
import {
  getAddress,
  type Address,
} from "viem";

const { viem } =
  await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error(
    "No wallet configured for this network",
  );
}

const wallet = wallets[0];

const chainId =
  await publicClient.getChainId();

const deploymentFile = new URL(
  `../ignition/deployments/chain-${chainId}/deployed_addresses.json`,
  import.meta.url,
);

const deployments =
  JSON.parse(
    readFileSync(
      deploymentFile,
      "utf8",
    ),
  ) as Record<string, string>;

const key =
  "PrismProofRegistryModule#PrismProofRegistry";

const rawAddress =
  deployments[key];

if (!rawAddress) {
  throw new Error(
    `PrismProofRegistry not found for chain ${chainId}`,
  );
}

const registryAddress =
  getAddress(rawAddress) as Address;

const registry =
  await viem.getContractAt(
    "PrismProofRegistry",
    registryAddress,
    {
      client: {
        public: publicClient,
        wallet,
      },
    },
  );

const owner =
  await registry.read.owner();

const recorder =
  await registry.read.recorders([
    wallet.account.address,
  ]);

const proofCount =
  await registry.read.proofCount();

console.log();
console.log("PRISM PROOF REGISTRY");
console.log("--------------------");
console.log("Chain ID    :", chainId);
console.log("Registry    :", registryAddress);
console.log("Wallet      :", wallet.account.address);
console.log("Owner       :", owner);
console.log("Is recorder :", recorder);
console.log("Proof count :", proofCount.toString());
console.log();

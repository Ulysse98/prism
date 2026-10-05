import { network } from "hardhat";
import { formatEther } from "viem";

const { viem } =
  await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error(
    "No Monad deployment wallet configured",
  );
}

const wallet =
  wallets[0];

const chainId =
  await publicClient.getChainId();

if (chainId !== 10143) {
  throw new Error(
    `Expected Monad Testnet chain 10143, got ${chainId}`,
  );
}

const balance =
  await publicClient.getBalance({
    address: wallet.account.address,
  });

console.log();
console.log("PRISM — MONAD DEPLOYER");
console.log("----------------------");
console.log("Chain ID :", chainId);
console.log("Address  :", wallet.account.address);
console.log("Balance  :", formatEther(balance), "MON");
console.log();
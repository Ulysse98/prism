import { network } from "hardhat";
import { formatEther } from "viem";

const { viem } = await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error(
    "No wallet configured for this network",
  );
}

const address =
  wallets[0].account.address;

const chainId =
  await publicClient.getChainId();

const balance =
  await publicClient.getBalance({
    address,
  });

console.log();
console.log("ARBITRUM SEPOLIA");
console.log("-----------------");
console.log("Chain ID :", chainId);
console.log("Wallet   :", address);
console.log(
  "Balance  :",
  formatEther(balance),
  "ETH",
);
console.log();
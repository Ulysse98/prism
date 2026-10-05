import hardhatToolboxViemPlugin from "@nomicfoundation/hardhat-toolbox-viem";
import { configVariable, defineConfig } from "hardhat/config";

export default defineConfig({
  plugins: [hardhatToolboxViemPlugin],

  solidity: {
    profiles: {
      default: {
        version: "0.8.34",
        settings: {
          evmVersion: "osaka",
        },
      },

      production: {
        version: "0.8.34",
        settings: {
          evmVersion: "osaka",
          optimizer: {
            enabled: true,
            runs: 200,
          },
        },
      },
    },
  },

  networks: {
    hardhatMainnet: {
      type: "edr-simulated",
      chainType: "l1",
    },

    monadTestnet: {
      type: "http",
      chainType: "generic",
      chainId: 10143,
      url: "https://testnet-rpc.monad.xyz",
      accounts: [
        configVariable("MONAD_PRIVATE_KEY"),
      ],
    },

    monadMainnet: {
      type: "http",
      chainId: 143,
      url: "https://rpc.monad.xyz",
      accounts: [
        configVariable("MONAD_PRIVATE_KEY"),
      ],
    },
  },
});

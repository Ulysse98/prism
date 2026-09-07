import "dotenv/config";

import {
  createAgentkitClient,
} from "@worldcoin/agentkit";

import {
  privateKeyToAccount,
} from "viem/accounts";

async function main() {
  const privateKey =
    process.env.AGENT_PRIVATE_KEY as
      | `0x${string}`
      | undefined;

  if (!privateKey) {
    throw new Error(
      "AGENT_PRIVATE_KEY missing from .env"
    );
  }

  const account =
    privateKeyToAccount(privateKey);

  const agentkit = createAgentkitClient({
    signer: {
      address: account.address,
      chainId: "eip155:480",
      type: "eip191",

      signMessage: (message) =>
        account.signMessage({ message }),
    },

    onEvent: (event) => {
      console.log(
        "[AgentKit]",
        event.type
      );
    },
  });

  console.log(
    "Agent:",
    account.address
  );

  console.log(
    "Requesting Prism PoUP access..."
  );

  const response =
    await agentkit.fetch(
      "http://127.0.0.1:4021/poup"
    );

  console.log();
  console.log(
    "HTTP status:",
    response.status
  );

  console.log(
    await response.text()
  );
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

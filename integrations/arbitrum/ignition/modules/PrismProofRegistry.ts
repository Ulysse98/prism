import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

export default buildModule("PrismProofRegistryModule", (m) => {
  const registry = m.contract("PrismProofRegistry");

  return { registry };
});

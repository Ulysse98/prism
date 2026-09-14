import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

export default buildModule("PrismWorkRegistryModule", (m) => {
  const registry = m.contract("PrismWorkRegistry");

  return { registry };
});
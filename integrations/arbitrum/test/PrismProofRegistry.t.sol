// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

import "../contracts/PrismProofRegistry.sol";

contract UnauthorizedRecorder {
    function attemptRegister(
        PrismProofRegistry registry,
        bytes32 jobId,
        bytes32 proofId,
        bytes32 workerIdHash,
        bytes32 prismChainIdHash
    ) external returns (bool success) {
        (success, ) = address(registry).call(
            abi.encodeWithSelector(
                registry.registerProofV2.selector,
                jobId,
                proofId,
                workerIdHash,
                prismChainIdHash
            )
        );
    }
}

contract PrismProofRegistryTest {
    PrismProofRegistry private registry;

    bytes32 private jobId;
    bytes32 private proofId;
    bytes32 private workerIdHash;
    bytes32 private prismChainIdHash;

    function setUp() public {
        registry = new PrismProofRegistry();

        jobId = keccak256(bytes("prism-job-001"));
        proofId = keccak256(bytes("prism-proof-v2-001"));
        workerIdHash = keccak256(bytes("prism-worker-alice"));
        prismChainIdHash = keccak256(
            bytes("prism-d8c1f3e740b48957")
        );
    }

    function testRegisterProofV2() public {
        bytes32 registryId = registry.registerProofV2(
            jobId,
            proofId,
            workerIdHash,
            prismChainIdHash
        );

        require(
            registry.hasProof(jobId),
            "proof should exist"
        );

        require(
            registry.proofCount() == 1,
            "proofCount should be 1"
        );

        PrismProofRegistry.ProofRecord memory record =
            registry.getProof(jobId);

        require(
            record.proofId == proofId,
            "wrong proofId"
        );

        require(
            record.workerIdHash == workerIdHash,
            "wrong workerIdHash"
        );

        require(
            record.submitter == address(this),
            "wrong submitter"
        );

        require(
            record.exists,
            "record should exist"
        );

        require(
            registry.getPrismChainIdHash(jobId)
                == prismChainIdHash,
            "wrong Prism chain ID"
        );

        require(
            registry.getRegistryId(jobId) == registryId,
            "wrong registry ID"
        );

        require(
            registry.getJobForRegistry(registryId) == jobId,
            "registry reverse lookup failed"
        );
    }

    function testRegistryIdIsDeterministic() public view {
        bytes32 expected = keccak256(
            abi.encode(
                jobId,
                prismChainIdHash,
                proofId
            )
        );

        bytes32 actual = registry.computeRegistryId(
            jobId,
            prismChainIdHash,
            proofId
        );

        require(
            actual == expected,
            "registry ID mismatch"
        );
    }

    function testRejectsDuplicateJob() public {
        registry.registerProofV2(
            jobId,
            proofId,
            workerIdHash,
            prismChainIdHash
        );

        bytes32 secondProofId =
            keccak256(bytes("prism-proof-v2-002"));

        (bool success, ) = address(registry).call(
            abi.encodeWithSelector(
                registry.registerProofV2.selector,
                jobId,
                secondProofId,
                workerIdHash,
                prismChainIdHash
            )
        );

        require(
            !success,
            "duplicate job should revert"
        );

        require(
            registry.proofCount() == 1,
            "duplicate changed proofCount"
        );
    }

    function testRejectsDuplicateProof() public {
        registry.registerProofV2(
            jobId,
            proofId,
            workerIdHash,
            prismChainIdHash
        );

        bytes32 secondJobId =
            keccak256(bytes("prism-job-002"));

        (bool success, ) = address(registry).call(
            abi.encodeWithSelector(
                registry.registerProofV2.selector,
                secondJobId,
                proofId,
                workerIdHash,
                prismChainIdHash
            )
        );

        require(
            !success,
            "duplicate proof should revert"
        );
    }

    function testRejectsUnauthorizedRecorder() public {
        UnauthorizedRecorder outsider =
            new UnauthorizedRecorder();

        bool success = outsider.attemptRegister(
            registry,
            jobId,
            proofId,
            workerIdHash,
            prismChainIdHash
        );

        require(
            !success,
            "unauthorized recorder should revert"
        );

        require(
            registry.proofCount() == 0,
            "unauthorized call changed state"
        );
    }

    function testLegacyRegistrationStillWorks() public {
        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );

        require(
            registry.hasProof(jobId),
            "legacy proof missing"
        );

        require(
            registry.getJobForProof(proofId) == jobId,
            "legacy reverse lookup failed"
        );

        require(
            registry.getPrismChainIdHash(jobId)
                == bytes32(0),
            "legacy proof should not have chain ID"
        );

        require(
            registry.getRegistryId(jobId)
                == bytes32(0),
            "legacy proof should not have registry ID"
        );
    }
}

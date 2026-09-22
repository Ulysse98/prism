// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

import {Test} from "forge-std/Test.sol";
import {PrismProofRegistry} from "./PrismProofRegistry.sol";

contract PrismProofRegistryTest is Test {
    PrismProofRegistry internal registry;

    address internal unauthorized = address(0xB0B);
    address internal recorder = address(0xCAFE);

    bytes32 internal jobId =
        sha256(bytes("prism-job-001"));

    bytes32 internal proofId =
        sha256(bytes("prism-proof-v2-001"));

    bytes32 internal workerIdHash =
        sha256(bytes("prism_worker_001"));

    function setUp() public {
        registry = new PrismProofRegistry();
    }

    function test_DeployerIsOwnerAndRecorder() public view {
        assertEq(registry.owner(), address(this));
        assertTrue(registry.recorders(address(this)));
    }

    function test_RegisterProof() public {
        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );

        PrismProofRegistry.ProofRecord memory proof =
            registry.getProof(jobId);

        assertEq(proof.proofId, proofId);
        assertEq(proof.workerIdHash, workerIdHash);
        assertEq(proof.submitter, address(this));
        assertTrue(proof.registeredAt > 0);
        assertTrue(proof.exists);

        assertTrue(registry.hasProof(jobId));
        assertEq(
            registry.getJobForProof(proofId),
            jobId
        );

        assertEq(registry.proofCount(), 1);
    }

    function test_UnauthorizedCannotRegister() public {
        vm.prank(unauthorized);

        vm.expectRevert(
            PrismProofRegistry.Unauthorized.selector
        );

        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );
    }

    function test_OwnerCanAuthorizeRecorder() public {
        registry.setRecorder(
            recorder,
            true
        );

        assertTrue(registry.recorders(recorder));

        vm.prank(recorder);

        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );

        PrismProofRegistry.ProofRecord memory proof =
            registry.getProof(jobId);

        assertEq(proof.submitter, recorder);
        assertTrue(proof.exists);
    }

    function test_OwnerCanRevokeRecorder() public {
        registry.setRecorder(
            recorder,
            true
        );

        registry.setRecorder(
            recorder,
            false
        );

        assertFalse(registry.recorders(recorder));

        vm.prank(recorder);

        vm.expectRevert(
            PrismProofRegistry.Unauthorized.selector
        );

        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );
    }

    function test_NonOwnerCannotAuthorizeRecorder() public {
        vm.prank(unauthorized);

        vm.expectRevert(
            PrismProofRegistry.Unauthorized.selector
        );

        registry.setRecorder(
            recorder,
            true
        );
    }

    function test_RejectsZeroJobId() public {
        vm.expectRevert(
            PrismProofRegistry.InvalidJobId.selector
        );

        registry.registerProof(
            bytes32(0),
            proofId,
            workerIdHash
        );
    }

    function test_RejectsZeroProofId() public {
        vm.expectRevert(
            PrismProofRegistry.InvalidProofId.selector
        );

        registry.registerProof(
            jobId,
            bytes32(0),
            workerIdHash
        );
    }

    function test_RejectsZeroWorkerIdHash() public {
        vm.expectRevert(
            PrismProofRegistry.InvalidWorkerIdHash.selector
        );

        registry.registerProof(
            jobId,
            proofId,
            bytes32(0)
        );
    }

    function test_RejectsDuplicateJob() public {
        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );

        bytes32 secondProofId =
            sha256(bytes("prism-proof-v2-002"));

        vm.expectRevert(
            abi.encodeWithSelector(
                PrismProofRegistry.JobAlreadyRegistered.selector,
                jobId
            )
        );

        registry.registerProof(
            jobId,
            secondProofId,
            workerIdHash
        );
    }

    function test_RejectsDuplicateProofForDifferentJob() public {
        registry.registerProof(
            jobId,
            proofId,
            workerIdHash
        );

        bytes32 secondJobId =
            sha256(bytes("prism-job-002"));

        vm.expectRevert(
            abi.encodeWithSelector(
                PrismProofRegistry.ProofAlreadyRegistered.selector,
                proofId
            )
        );

        registry.registerProof(
            secondJobId,
            proofId,
            workerIdHash
        );
    }
}

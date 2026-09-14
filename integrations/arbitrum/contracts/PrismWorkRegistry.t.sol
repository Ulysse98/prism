// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

import {Test} from "forge-std/Test.sol";
import {PrismWorkRegistry} from "./PrismWorkRegistry.sol";

contract PrismWorkRegistryTest is Test {
    PrismWorkRegistry internal registry;

    string internal prismWorker =
        "prism_91e09dda8b18ca23de50770a03a1d32824802f2b";

    address internal unauthorized =
        address(0xB0B);

    function setUp() public {
        registry = new PrismWorkRegistry();
    }

    function test_DeployerIsRecorder() public view {
        assertTrue(
            registry.recorders(address(this))
        );
    }

    function test_RecordUsefulWork() public {
        bytes32 proofId =
            sha256(bytes("prism-pouw-proof-001"));

        registry.recordUsefulWork(
            prismWorker,
            proofId,
            "sum_squares",
            2
        );

        PrismWorkRegistry.UsefulWorkProof memory proof =
            registry.getProof(proofId);

        assertEq(proof.prismWorker, prismWorker);
        assertEq(proof.proofId, proofId);
        assertEq(proof.workType, "sum_squares");
        assertEq(proof.score, 2);

        assertTrue(proof.anchoredAt > 0);
        assertTrue(registry.proofExists(proofId));
        assertEq(registry.proofCount(), 1);
    }

    function test_CannotRecordDuplicateProof() public {
        bytes32 proofId =
            sha256(bytes("prism-pouw-proof-002"));

        registry.recordUsefulWork(
            prismWorker,
            proofId,
            "prime_count",
            5
        );

        vm.expectRevert(
            abi.encodeWithSelector(
                PrismWorkRegistry.ProofAlreadyExists.selector,
                proofId
            )
        );

        registry.recordUsefulWork(
            prismWorker,
            proofId,
            "prime_count",
            5
        );
    }

    function test_UnauthorizedCannotRecord() public {
        bytes32 proofId =
            sha256(bytes("prism-pouw-proof-003"));

        vm.prank(unauthorized);

        vm.expectRevert(
            PrismWorkRegistry.Unauthorized.selector
        );

        registry.recordUsefulWork(
            prismWorker,
            proofId,
            "matrix_multiply",
            10
        );
    }

    function test_OwnerCanAuthorizeRecorder() public {
        registry.setRecorder(
            unauthorized,
            true
        );

        assertTrue(
            registry.recorders(unauthorized)
        );

        bytes32 proofId =
            sha256(bytes("prism-pouw-proof-004"));

        vm.prank(unauthorized);

        registry.recordUsefulWork(
            prismWorker,
            proofId,
            "dot_product",
            6
        );

        assertTrue(
            registry.proofExists(proofId)
        );
    }

    function test_RejectsEmptyPrismWorker() public {
        bytes32 proofId =
            sha256(bytes("prism-pouw-proof-005"));

        vm.expectRevert(
            PrismWorkRegistry.InvalidWorker.selector
        );

        registry.recordUsefulWork(
            "",
            proofId,
            "sum_squares",
            2
        );
    }

    function test_RejectsZeroProofId() public {
        vm.expectRevert(
            PrismWorkRegistry.InvalidProofId.selector
        );

        registry.recordUsefulWork(
            prismWorker,
            bytes32(0),
            "sum_squares",
            2
        );
    }
}
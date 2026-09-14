// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

import {Test} from "forge-std/Test.sol";
import {PrismWorkRegistry} from "./PrismWorkRegistry.sol";

contract PrismWorkRegistryTest is Test {
    PrismWorkRegistry internal registry;

    address internal worker =
        address(0xA11CE);

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
        bytes32 proofHash =
            keccak256("prism-pouw-proof-001");

        registry.recordUsefulWork(
            worker,
            proofHash,
            "sum_squares",
            2
        );

        PrismWorkRegistry.UsefulWorkProof memory proof =
            registry.getProof(proofHash);

        assertEq(proof.worker, worker);
        assertEq(proof.proofHash, proofHash);
        assertEq(proof.workType, "sum_squares");
        assertEq(proof.score, 2);

        assertTrue(proof.timestamp > 0);
        assertTrue(registry.proofExists(proofHash));
        assertEq(registry.proofCount(), 1);
    }

    function test_CannotRecordDuplicateProof() public {
        bytes32 proofHash =
            keccak256("prism-pouw-proof-002");

        registry.recordUsefulWork(
            worker,
            proofHash,
            "prime_search",
            5
        );

        vm.expectRevert(
            abi.encodeWithSelector(
                PrismWorkRegistry.ProofAlreadyExists.selector,
                proofHash
            )
        );

        registry.recordUsefulWork(
            worker,
            proofHash,
            "prime_search",
            5
        );
    }

    function test_UnauthorizedCannotRecord() public {
        bytes32 proofHash =
            keccak256("prism-pouw-proof-003");

        vm.prank(unauthorized);

        vm.expectRevert(
            PrismWorkRegistry.Unauthorized.selector
        );

        registry.recordUsefulWork(
            worker,
            proofHash,
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

        bytes32 proofHash =
            keccak256("prism-pouw-proof-004");

        vm.prank(unauthorized);

        registry.recordUsefulWork(
            worker,
            proofHash,
            "hash_search",
            3
        );

        assertTrue(
            registry.proofExists(proofHash)
        );
    }
}
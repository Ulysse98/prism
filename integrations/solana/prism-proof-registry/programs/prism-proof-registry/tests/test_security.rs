use {
    anchor_lang::{
        prelude::Pubkey,
        solana_program::{instruction::Instruction, system_program},
        AccountDeserialize, InstructionData, ToAccountMetas,
    },
    litesvm::LiteSVM,
    solana_keypair::Keypair,
    solana_message::{Message, VersionedMessage},
    solana_signer::Signer,
    solana_transaction::versioned::VersionedTransaction,
};

fn execute_ix(svm: &mut LiteSVM, payer: &Keypair, instruction: Instruction) -> bool {
    let blockhash = svm.latest_blockhash();

    let message = Message::new_with_blockhash(&[instruction], Some(&payer.pubkey()), &blockhash);

    let tx = VersionedTransaction::try_new(VersionedMessage::Legacy(message), &[payer]).unwrap();

    svm.send_transaction(tx).is_ok()
}

fn initialize_registry() -> (LiteSVM, Keypair, Pubkey) {
    let program_id = prism_proof_registry::id();
    let owner = Keypair::new();

    let (config, _) = Pubkey::find_program_address(&[b"registry"], &program_id);

    let mut svm = LiteSVM::new();

    let program_bytes = include_bytes!(concat!(
        env!("CARGO_TARGET_TMPDIR"),
        "/../deploy/prism_proof_registry.so"
    ));

    svm.add_program(program_id, program_bytes).unwrap();

    svm.airdrop(&owner.pubkey(), 10_000_000_000).unwrap();

    let ix = Instruction::new_with_bytes(
        program_id,
        &prism_proof_registry::instruction::InitializeRegistry {}.data(),
        prism_proof_registry::accounts::InitializeRegistry {
            config,
            owner: owner.pubkey(),
            system_program: system_program::ID,
        }
        .to_account_metas(None),
    );

    assert!(
        execute_ix(&mut svm, &owner, ix),
        "registry initialization must succeed"
    );

    (svm, owner, config)
}

fn register_proof_ix(
    config: Pubkey,
    recorder: Pubkey,
    job_id: [u8; 32],
    proof_id: [u8; 32],
    worker_id_hash: [u8; 32],
    prism_chain_id_hash: [u8; 32],
) -> Instruction {
    let program_id = prism_proof_registry::id();

    let (proof, _) = Pubkey::find_program_address(&[b"job", job_id.as_ref()], &program_id);

    let (proof_index, _) =
        Pubkey::find_program_address(&[b"proof", proof_id.as_ref()], &program_id);

    Instruction::new_with_bytes(
        program_id,
        &prism_proof_registry::instruction::RegisterProofV2 {
            job_id,
            proof_id,
            worker_id_hash,
            prism_chain_id_hash,
        }
        .data(),
        prism_proof_registry::accounts::RegisterProofV2 {
            config,
            proof,
            proof_index,
            recorder,
            system_program: system_program::ID,
        }
        .to_account_metas(None),
    )
}

fn proof_count(svm: &LiteSVM, config: &Pubkey) -> u64 {
    let account = svm.get_account(config).expect("registry config must exist");

    let mut data: &[u8] = &account.data;

    let state = prism_proof_registry::RegistryConfig::try_deserialize(&mut data).unwrap();

    state.proof_count
}

#[test]
fn rejects_duplicate_job_id() {
    let (mut svm, owner, config) = initialize_registry();

    let job_id = [1u8; 32];

    let first = register_proof_ix(
        config,
        owner.pubkey(),
        job_id,
        [2u8; 32],
        [3u8; 32],
        [4u8; 32],
    );

    assert!(
        execute_ix(&mut svm, &owner, first),
        "first proof must succeed"
    );

    let duplicate = register_proof_ix(
        config,
        owner.pubkey(),
        job_id,
        [5u8; 32],
        [6u8; 32],
        [4u8; 32],
    );

    assert!(
        !execute_ix(&mut svm, &owner, duplicate),
        "duplicate job ID must be rejected"
    );

    assert_eq!(proof_count(&svm, &config), 1);

    let (unused_index, _) =
        Pubkey::find_program_address(&[b"proof", [5u8; 32].as_ref()], &prism_proof_registry::id());

    assert!(
        svm.get_account(&unused_index).is_none(),
        "failed transaction must not leave a proof index"
    );
}

#[test]
fn rejects_duplicate_proof_id() {
    let (mut svm, owner, config) = initialize_registry();

    let proof_id = [2u8; 32];

    let first = register_proof_ix(
        config,
        owner.pubkey(),
        [1u8; 32],
        proof_id,
        [3u8; 32],
        [4u8; 32],
    );

    assert!(
        execute_ix(&mut svm, &owner, first),
        "first proof must succeed"
    );

    let second_job_id = [5u8; 32];

    let duplicate = register_proof_ix(
        config,
        owner.pubkey(),
        second_job_id,
        proof_id,
        [6u8; 32],
        [4u8; 32],
    );

    assert!(
        !execute_ix(&mut svm, &owner, duplicate),
        "duplicate proof ID must be rejected"
    );

    assert_eq!(proof_count(&svm, &config), 1);

    let (unused_proof, _) = Pubkey::find_program_address(
        &[b"job", second_job_id.as_ref()],
        &prism_proof_registry::id(),
    );

    assert!(
        svm.get_account(&unused_proof).is_none(),
        "failed transaction must not leave a proof record"
    );
}

#[test]
fn rejects_unauthorized_recorder() {
    let (mut svm, _owner, config) = initialize_registry();

    let attacker = Keypair::new();

    svm.airdrop(&attacker.pubkey(), 10_000_000_000).unwrap();

    let job_id = [7u8; 32];
    let proof_id = [8u8; 32];

    let unauthorized = register_proof_ix(
        config,
        attacker.pubkey(),
        job_id,
        proof_id,
        [9u8; 32],
        [10u8; 32],
    );

    assert!(
        !execute_ix(&mut svm, &attacker, unauthorized,),
        "unauthorized recorder must be rejected"
    );

    assert_eq!(proof_count(&svm, &config), 0);

    let (proof, _) =
        Pubkey::find_program_address(&[b"job", job_id.as_ref()], &prism_proof_registry::id());

    let (proof_index, _) =
        Pubkey::find_program_address(&[b"proof", proof_id.as_ref()], &prism_proof_registry::id());

    assert!(svm.get_account(&proof).is_none());
    assert!(svm.get_account(&proof_index).is_none());
}

fn assert_rejected_without_state(
    svm: &mut LiteSVM,
    recorder: &Keypair,
    config: Pubkey,
    job_id: [u8; 32],
    proof_id: [u8; 32],
    worker_id_hash: [u8; 32],
    prism_chain_id_hash: [u8; 32],
) {
    let ix = register_proof_ix(
        config,
        recorder.pubkey(),
        job_id,
        proof_id,
        worker_id_hash,
        prism_chain_id_hash,
    );

    assert!(
        !execute_ix(svm, recorder, ix),
        "invalid proof input must be rejected"
    );

    assert_eq!(proof_count(svm, &config), 0);

    let (proof, _) =
        Pubkey::find_program_address(&[b"job", job_id.as_ref()], &prism_proof_registry::id());

    let (proof_index, _) =
        Pubkey::find_program_address(&[b"proof", proof_id.as_ref()], &prism_proof_registry::id());

    assert!(
        svm.get_account(&proof).is_none(),
        "failed transaction must not leave a proof record"
    );

    assert!(
        svm.get_account(&proof_index).is_none(),
        "failed transaction must not leave a proof index"
    );
}

#[test]
fn rejects_zero_job_id() {
    let (mut svm, owner, config) = initialize_registry();

    assert_rejected_without_state(
        &mut svm, &owner, config, [0u8; 32], [2u8; 32], [3u8; 32], [4u8; 32],
    );
}

#[test]
fn rejects_zero_proof_id() {
    let (mut svm, owner, config) = initialize_registry();

    assert_rejected_without_state(
        &mut svm, &owner, config, [1u8; 32], [0u8; 32], [3u8; 32], [4u8; 32],
    );
}

#[test]
fn rejects_zero_worker_id_hash() {
    let (mut svm, owner, config) = initialize_registry();

    assert_rejected_without_state(
        &mut svm, &owner, config, [1u8; 32], [2u8; 32], [0u8; 32], [4u8; 32],
    );
}

#[test]
fn rejects_zero_prism_chain_id_hash() {
    let (mut svm, owner, config) = initialize_registry();

    assert_rejected_without_state(
        &mut svm, &owner, config, [1u8; 32], [2u8; 32], [3u8; 32], [0u8; 32],
    );
}

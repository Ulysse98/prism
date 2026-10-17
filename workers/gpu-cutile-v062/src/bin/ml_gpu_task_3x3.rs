use cutile::prelude::*;
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::sync::Arc;
use std::time::Instant;

#[cutile::module]
mod ml_gpu {
    use cutile::core::*;

    #[cutile::entry()]
    fn predict<const S: [i32; 1]>(
        out: &mut Tensor<i64, S>,
        x0: &Tensor<i64, { [-1] }>,
        x1: &Tensor<i64, { [-1] }>,
        x2: &Tensor<i64, { [-1] }>,
        w00: &Tensor<i64, { [-1] }>,
        w01: &Tensor<i64, { [-1] }>,
        w02: &Tensor<i64, { [-1] }>,
        w10: &Tensor<i64, { [-1] }>,
        w11: &Tensor<i64, { [-1] }>,
        w12: &Tensor<i64, { [-1] }>,
        w20: &Tensor<i64, { [-1] }>,
        w21: &Tensor<i64, { [-1] }>,
        w22: &Tensor<i64, { [-1] }>,
        b0: &Tensor<i64, { [-1] }>,
        b1: &Tensor<i64, { [-1] }>,
        b2: &Tensor<i64, { [-1] }>,
    ) {
        let shape = out.shape();

        let a = x0.load_like(out);
        let b = x1.load_like(out);
        let c = x2.load_like(out);

        let s0 = a * w00.load_like(out)
            + b * w01.load_like(out)
            + c * w02.load_like(out)
            + b0.load_like(out);

        let s1 = a * w10.load_like(out)
            + b * w11.load_like(out)
            + c * w12.load_like(out)
            + b1.load_like(out);

        let s2 = a * w20.load_like(out)
            + b * w21.load_like(out)
            + c * w22.load_like(out)
            + b2.load_like(out);

        // Strict > preserves the lowest class on ties.
        let choose1 = gt_tile(s1, s0);

        let best01 = select(choose1, s1, s0);

        let class01 = select(
            gt_tile(s1, s0),
            1i64.broadcast(shape),
            0i64.broadcast(shape),
        );

        let prediction = select(gt_tile(s2, best01), 2i64.broadcast(shape), class01);

        out.store(prediction);
    }
}

fn sha256(data: &[u8]) -> String {
    format!("{:x}", Sha256::digest(data))
}

fn read_i64(v: &Value, key: &str) -> Vec<i64> {
    v[key]
        .as_array()
        .expect("missing array")
        .iter()
        .map(|x| x.as_i64().expect("invalid i64"))
        .collect()
}

// This worker receives ONE Prism task via stdin.
// Unlike the fixture harness, it does not receive
// expected predictions and does not compute argmax on CPU.

fn ensure_safe_integer_ranges(inputs: &[i64], weights: &[i64], biases: &[i64]) {
    let mut max_abs = [0i128; 3];

    for sample in inputs.chunks_exact(3) {
        for feature in 0..3 {
            let value = (sample[feature] as i128).abs();

            max_abs[feature] = max_abs[feature].max(value);
        }
    }

    let limit = i64::MAX as i128;

    for class in 0..3 {
        let mut bound = (biases[class] as i128).abs();

        assert!(bound <= limit, "unsafe bias range");

        for feature in 0..3 {
            let weight = (weights[class * 3 + feature] as i128).abs();

            let term = max_abs[feature]
                .checked_mul(weight)
                .expect("integer range multiplication overflow");

            assert!(term <= limit, "unsafe product range");

            bound = bound
                .checked_add(term)
                .expect("integer range addition overflow");

            assert!(bound <= limit, "unsafe score range");
        }
    }
}

fn main() -> Result<(), Error> {
    use std::io::Read;

    let worker_started = Instant::now();
    let profile_enabled = matches!(std::env::var("PRISM_GPU_PROFILE").as_deref(), Ok("1"));

    const MAX_TASK_JSON_BYTES: u64 = 512 * 1024;

    let mut input = String::new();

    std::io::stdin()
        .take(MAX_TASK_JSON_BYTES + 1)
        .read_to_string(&mut input)
        .expect("cannot read task from stdin");

    assert!(
        input.len() as u64 <= MAX_TASK_JSON_BYTES,
        "task JSON exceeds 512 KiB"
    );

    let envelope: Value = serde_json::from_str(&input).expect("invalid JSON");

    let task = envelope.get("task").expect("missing task");

    assert_eq!(
        task["type"].as_str(),
        Some("ml_inference_quantized"),
        "unsupported task type",
    );

    let batch = usize::try_from(task["rows_a"].as_u64().expect("invalid batch"))
        .expect("batch exceeds platform size");

    assert!(batch > 0 && batch <= 4096, "unsupported batch size",);

    assert_eq!(task["cols_a"].as_u64(), Some(3));
    assert_eq!(task["cols_b"].as_u64(), Some(3));

    let inputs = read_i64(task, "signed_values");
    let weights = read_i64(task, "signed_values_b");
    let biases = read_i64(task, "biases");

    assert_eq!(inputs.len(), batch * 3);
    assert_eq!(weights.len(), 9);
    assert_eq!(biases.len(), 3);

    // Go computes scores with checked int64 operations.
    // Conservative bounds reject any potentially unsafe
    // GPU integer arithmetic without calculating predictions.
    ensure_safe_integer_ranges(&inputs, &weights, &biases);

    let payload = format!(
        "{{\"batch_size\":{},\"features\":3,\
         \"classes\":3,\"inputs\":{},\
         \"weights\":{},\"biases\":{}}}",
        batch,
        serde_json::to_string(&inputs).unwrap(),
        serde_json::to_string(&weights).unwrap(),
        serde_json::to_string(&biases).unwrap(),
    );

    let input_hash = sha256(payload.as_bytes());

    assert_eq!(
        task["input_hash"].as_str(),
        Some(input_hash.as_str()),
        "input hash mismatch",
    );

    let task_id = sha256(format!("ml_inference_quantized|{input_hash}").as_bytes());

    assert_eq!(
        task["id"].as_str(),
        Some(task_id.as_str()),
        "task ID mismatch",
    );

    let parsing_finished = Instant::now();
    let device = Device::new(0)?;
    let stream = device.new_stream()?;
    let device_ready = Instant::now();

    let mut columns = [
        Vec::with_capacity(batch),
        Vec::with_capacity(batch),
        Vec::with_capacity(batch),
    ];

    for sample in inputs.chunks_exact(3) {
        for feature in 0..3 {
            columns[feature].push(sample[feature]);
        }
    }
    let columns_ready = Instant::now();

    let x0: Arc<Tensor<i64>> = api::copy_host_vec_to_device(&Arc::new(columns[0].clone()))
        .sync_on(&stream)?
        .into();

    let x1: Arc<Tensor<i64>> = api::copy_host_vec_to_device(&Arc::new(columns[1].clone()))
        .sync_on(&stream)?
        .into();

    let x2: Arc<Tensor<i64>> = api::copy_host_vec_to_device(&Arc::new(columns[2].clone()))
        .sync_on(&stream)?
        .into();
    // v0.63: model values are runtime GPU data, not scalar JIT parameters.
    // Conservative implementation: 12 vectors repeated across each batch.
    // Optimize representation only after production-path correctness tests.
    let model_values: Vec<i64> = weights.iter().chain(biases.iter()).copied().collect();
    assert_eq!(model_values.len(), 12);
    let mut model_tensors: Vec<Arc<Tensor<i64>>> = Vec::with_capacity(12);
    for &value in model_values.iter() {
        let repeated = Arc::new(vec![value; batch]);
        let tensor: Arc<Tensor<i64>> = api::copy_host_vec_to_device(&repeated)
            .sync_on(&stream)?
            .into();
        model_tensors.push(tensor);
    }
    let h2d_finished = Instant::now();

    let out = api::zeros::<i64>(&[batch])
        .sync_on(&stream)?
        .partition([32]);
    let output_allocated = Instant::now();

    let outputs = ml_gpu::predict(
        out,
        x0,
        x1,
        x2,
        model_tensors[0].clone(),
        model_tensors[1].clone(),
        model_tensors[2].clone(),
        model_tensors[3].clone(),
        model_tensors[4].clone(),
        model_tensors[5].clone(),
        model_tensors[6].clone(),
        model_tensors[7].clone(),
        model_tensors[8].clone(),
        model_tensors[9].clone(),
        model_tensors[10].clone(),
        model_tensors[11].clone(),
    )
    .sync_on(&stream)?;
    let kernel_finished = Instant::now();

    let gpu_values: Vec<i64> = outputs.0.unpartition().to_host_vec().sync_on(&stream)?;
    let d2h_finished = Instant::now();

    let predictions: Vec<u64> = gpu_values
        .into_iter()
        .map(|value| u64::try_from(value).unwrap())
        .collect();

    assert_eq!(predictions.len(), batch);

    for class in &predictions {
        assert!(*class < 3, "invalid GPU prediction");
    }

    let result_json = serde_json::to_vec(&predictions).expect("prediction serialization failed");

    let output_hash = sha256(&result_json);

    let receipt = serde_json::json!({
        "schema": "prism-gpu-ml-lab-v0",
        "task_id": task_id,
        "input_hash": input_hash,
        "result_values": predictions,
        "output_hash": output_hash,
        "gpu_attestation": false
    });

    let receipt_line = format!("PRISM_GPU_RECEIPT={receipt}");
    let receipt_ready = Instant::now();
    println!("{receipt_line}");

    if profile_enabled {
        // Host wall-clock timers synchronized with CUDA operations; the
        // kernel region may include JIT compilation and launch overhead.
        // Never include timings in the consensus-critical stdout receipt.
        let millis = |duration: std::time::Duration| duration.as_secs_f64() * 1000.0;
        let profile = serde_json::json!({
            "schema": "prism-gpu-profile-v063",
            "measurement": "host_instant_sync_walltime_not_kernel_only",
            "batch_size": batch,
            "host_parse_validation_ms": millis(parsing_finished.duration_since(worker_started)),
            "device_stream_init_ms": millis(device_ready.duration_since(parsing_finished)),
            "host_column_prep_ms": millis(columns_ready.duration_since(device_ready)),
            "h2d_transfer_sync_ms": millis(h2d_finished.duration_since(columns_ready)),
            "device_output_alloc_ms": millis(output_allocated.duration_since(h2d_finished)),
            "kernel_launch_jit_sync_ms": millis(kernel_finished.duration_since(output_allocated)),
            "d2h_transfer_sync_ms": millis(d2h_finished.duration_since(kernel_finished)),
            "host_result_receipt_ms": millis(receipt_ready.duration_since(d2h_finished)),
            "total_worker_ms": millis(receipt_ready.duration_since(worker_started)),
        });
        eprintln!("PRISM_GPU_PROFILE={profile}");
    }

    Ok(())
}

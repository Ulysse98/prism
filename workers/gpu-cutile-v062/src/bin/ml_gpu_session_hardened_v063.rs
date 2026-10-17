use cutile::prelude::*;
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::io::{self, Write};
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

// Experimental persistent IPC: newline-delimited seq, nonce and Prism task.
// No expected CPU predictions are supplied. Each response carries the existing
// six-field PoUW receipt inside a session-only nonce-bound envelope.

// EXPERIMENTAL: bounded, newline-delimited IPC reader; never buffers >512KiB
// of untrusted request bytes. Reject unterminated lines and invalid UTF-8.
fn read_bounded_gpu_request<R: io::BufRead>(
    reader: &mut R,
    line: &mut String,
) -> io::Result<usize> {
    const MAX_REQUEST_BYTES: usize = 512 * 1024;
    let mut frame = Vec::<u8>::new();
    loop {
        let (used, has_newline) = {
            let available = reader.fill_buf()?;
            if available.is_empty() {
                if frame.is_empty() {
                    return Ok(0);
                }
                return Err(io::Error::new(
                    io::ErrorKind::UnexpectedEof,
                    "incomplete GPU request frame",
                ));
            }
            let used = available
                .iter()
                .position(|&byte| byte == b'\n')
                .map_or(available.len(), |i| i + 1);
            if used > MAX_REQUEST_BYTES.saturating_sub(frame.len()) {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidData,
                    "GPU request frame exceeds 512 KiB",
                ));
            }
            frame.extend_from_slice(&available[..used]);
            (used, available[used - 1] == b'\n')
        };
        reader.consume(used);
        if has_newline {
            *line = String::from_utf8(frame)
                .map_err(|_| io::Error::new(io::ErrorKind::InvalidData, "invalid request UTF-8"))?;
            return Ok(line.len());
        }
    }
}

fn main() -> Result<(), Error> {
    let setup_start = Instant::now();
    let device = Device::new(0)?;
    let stream = device.new_stream()?;
    let setup_ms = setup_start.elapsed().as_secs_f64() * 1000.0;
    let profile_enabled = matches!(std::env::var("PRISM_GPU_PROFILE").as_deref(), Ok("1"));
    if profile_enabled {
        eprintln!(
            "PRISM_MULTITASK_SETUP={}",
            serde_json::json!({
                "schema": "prism-gpu-multi-setup-v063", "device_stream_init_ms": setup_ms,
            })
        );
    }

    let stdin = io::stdin();
    let mut reader = stdin.lock();
    let mut line = String::new();
    let mut count = 0usize;
    loop {
        line.clear();
        if read_bounded_gpu_request(&mut reader, &mut line).expect("cannot read request") == 0 {
            break;
        }
        // Size is checked incrementally by read_bounded_gpu_request.
        if line.trim().is_empty() {
            continue;
        }
        // Reject the 65th task BEFORE any GPU execution or receipt output.
        assert!(count < 64, "session task limit reached");
        let started = Instant::now();
        let envelope: Value = serde_json::from_str(&line).expect("invalid benchmark request JSON");
        let seq = envelope["seq"].as_u64().expect("missing sequence");
        // Session-only freshness key: never put it inside the consensus receipt.
        let nonce = envelope["nonce"].as_str().expect("missing request nonce");
        assert!(
            nonce.len() == 64
                && nonce
                    .bytes()
                    .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b)),
            "invalid session nonce"
        );
        let task = envelope.get("task").expect("missing task");
        assert_eq!(task["type"].as_str(), Some("ml_inference_quantized"));
        let batch = usize::try_from(task["rows_a"].as_u64().expect("batch missing"))
            .expect("batch too large");
        // Align with Go's maxMatrixElements=4096 and 3 input features:
        // batch*3 <= 4096, hence batch <= 1365.
        assert!(batch > 0 && batch <= 4096 / 3, "unsupported batch size");
        assert_eq!(task["cols_a"].as_u64(), Some(3));
        assert_eq!(task["cols_b"].as_u64(), Some(3));
        let inputs = read_i64(task, "signed_values");
        let weights = read_i64(task, "signed_values_b");
        let biases = read_i64(task, "biases");
        assert_eq!(inputs.len(), batch * 3);
        assert_eq!(weights.len(), 9);
        assert_eq!(biases.len(), 3);
        ensure_safe_integer_ranges(&inputs, &weights, &biases);
        let input_payload = format!(
            "{{\"batch_size\":{},\"features\":3,\"classes\":3,\"inputs\":{},\"weights\":{},\"biases\":{}}}",
            batch, serde_json::to_string(&inputs).unwrap(),
            serde_json::to_string(&weights).unwrap(),
            serde_json::to_string(&biases).unwrap(),
        );
        let input_hash = sha256(input_payload.as_bytes());
        assert_eq!(
            task["input_hash"].as_str(),
            Some(input_hash.as_str()),
            "bad input hash"
        );
        let task_id = sha256(format!("ml_inference_quantized|{input_hash}").as_bytes());
        assert_eq!(task["id"].as_str(), Some(task_id.as_str()), "bad task ID");
        let parsed = Instant::now();

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
        // A/B EXPERIMENT: dynamic model tensors, not compile-time scalar values.
        // The repeated vector representation is intentionally simple; it adds
        // host-to-device work that the old scalar variant did not require.
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
        let h2d_ready = Instant::now();
        let out = api::zeros::<i64>(&[batch])
            .sync_on(&stream)?
            .partition([32]);
        let alloc_ready = Instant::now();
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
        let kernel_ready = Instant::now();
        let host_values: Vec<i64> = outputs.0.unpartition().to_host_vec().sync_on(&stream)?;
        let d2h_ready = Instant::now();
        let predictions: Vec<u64> = host_values
            .into_iter()
            .map(|v| {
                let class = u64::try_from(v).expect("negative GPU class");
                assert!(class < 3, "invalid GPU class");
                class
            })
            .collect();
        assert_eq!(predictions.len(), batch);
        let output_hash = sha256(&serde_json::to_vec(&predictions).unwrap());
        let receipt = serde_json::json!({
            "schema":"prism-gpu-ml-lab-v0",
            "task_id":task_id, "input_hash":input_hash,
            "result_values":predictions, "output_hash":output_hash,
            "gpu_attestation":false,
        });
        let finished = Instant::now();
        let ms = |start: Instant, end: Instant| end.duration_since(start).as_secs_f64() * 1000.0;
        let profile = serde_json::json!({
            "schema":"prism-gpu-multi-profile-v063", "seq":seq,
            "batch_size":batch,
            "parse_validate_ms":ms(started,parsed),
            "host_columns_ms":ms(parsed,columns_ready),
            "h2d_sync_ms":ms(columns_ready,h2d_ready),
            "alloc_sync_ms":ms(h2d_ready,alloc_ready),
            "kernel_jit_sync_ms":ms(alloc_ready,kernel_ready),
            "d2h_sync_ms":ms(kernel_ready,d2h_ready),
            "receipt_ms":ms(d2h_ready,finished),
            "task_total_ms":ms(started,finished),
        });
        // Do not exhaust Go's 16-KiB session-wide stderr quota by default.
        if profile_enabled {
            eprintln!("PRISM_MULTITASK_TIMING={profile}");
        }
        let session_response = serde_json::json!({
            "schema": "prism-gpu-session-frame-v063",
            "seq": seq,
            "nonce": nonce,
            "receipt": receipt,
        });
        println!("PRISM_GPU_SESSION={session_response}");
        io::stdout().flush().expect("cannot flush stdout");
        count += 1;
    }
    // A supervisor may open and close an idle session without any tasks.
    Ok(())
}

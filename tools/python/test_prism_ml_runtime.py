import copy
import hashlib
import json
from pathlib import Path
import unittest

from prism_ml_runtime import (
    InferenceError, INT64_MAX, INT64_MIN, canonical_input_bytes,
    infer, input_hash, load_payload,
)


def payload(inputs, weights, biases, batch=1, features=1, classes=1):
    return dict(batch_size=batch, features=features, classes=classes,
                inputs=inputs, weights=weights, biases=biases)


class QuantizedInferenceTests(unittest.TestCase):
    def test_signed_batch(self):
        task = json.loads(Path(__file__).with_name("example_quantized.json").read_text())
        self.assertEqual(infer(task)["predictions"], [0, 1, 0])

    def test_class_major_weight_layout(self):
        task = payload([3, 4], [1, 0, 0, 2, -1, -1], [0, 0, 0], features=2, classes=3)
        self.assertEqual(infer(task)["predictions"], [1])

    def test_negative_scores(self):
        task = payload([2], [-10, -2], [-1, -3], classes=2)
        self.assertEqual(infer(task)["predictions"], [1])

    def test_ties_choose_lowest_index(self):
        task = payload([5], [-2, -2, -2], [1, 1, 1], classes=3)
        self.assertEqual(infer(task)["predictions"], [0])

    def test_bias_decides(self):
        task = payload([0], [1, 1], [-1, 1], classes=2)
        self.assertEqual(infer(task)["predictions"], [1])

    def test_int64_boundaries_and_zero(self):
        for value in (INT64_MIN, INT64_MAX):
            for weight in (0, 1):
                with self.subTest(value=value, weight=weight):
                    self.assertEqual(infer(payload([value], [weight], [0]))["predictions"], [0])

    def test_multiplication_overflow(self):
        for a, b in ((INT64_MAX, 2), (INT64_MIN, -1), (-1, INT64_MIN), (INT64_MIN, 2)):
            with self.subTest(a=a, b=b), self.assertRaisesRegex(InferenceError, "multiplication overflow"):
                infer(payload([a], [b], [0]))

    def test_addition_overflow(self):
        for value, bias in ((1, INT64_MAX), (-1, INT64_MIN)):
            with self.subTest(value=value), self.assertRaisesRegex(InferenceError, "addition overflow"):
                infer(payload([value], [1], [bias]))

    def test_intermediate_overflow_even_if_final_sum_would_fit(self):
        for inputs, bias in (([1, -1], INT64_MAX), ([-1, 1], INT64_MIN)):
            with self.subTest(inputs=inputs), self.assertRaisesRegex(InferenceError, "addition overflow"):
                infer(payload(inputs, [1, 1], [bias], features=2))

    def test_losing_class_overflow_is_not_skipped(self):
        with self.assertRaisesRegex(InferenceError, "multiplication overflow"):
            infer(payload([INT64_MIN], [0, -1], [10, 0], classes=2))

    def test_exact_hash_payload_bytes(self):
        task = payload([2, -1], [1, -2, -1, 2], [0, -3], features=2, classes=2)
        expected = b'{"batch_size":1,"features":2,"classes":2,"inputs":[2,-1],"weights":[1,-2,-1,2],"biases":[0,-3]}'
        self.assertEqual(canonical_input_bytes(task), expected)
        self.assertEqual(input_hash(task), hashlib.sha256(expected).hexdigest())

    def test_hash_independent_of_incoming_key_order(self):
        task = payload([2], [3], [0])
        self.assertEqual(input_hash(task), input_hash(dict(reversed(list(task.items())))))

    def test_hash_commits_to_bias(self):
        self.assertNotEqual(input_hash(payload([2], [3], [0])), input_hash(payload([2], [3], [1])))

    def test_reject_non_integer_and_out_of_range_values(self):
        for field in ("inputs", "weights", "biases"):
            for value in (True, False, 1.0, "1", None, INT64_MAX + 1, INT64_MIN - 1):
                task = payload([1], [1], [0])
                task[field] = [value]
                with self.subTest(field=field, value=value), self.assertRaises(InferenceError):
                    infer(task)

    def test_invalid_dimensions(self):
        for field in ("batch_size", "features", "classes"):
            for value in (0, -1, True, 1.0, "1", 1 << 64):
                task = payload([1], [1], [0])
                task[field] = value
                with self.subTest(field=field, value=value), self.assertRaises(InferenceError):
                    infer(task)

    def test_dimension_product_overflow(self):
        with self.assertRaisesRegex(InferenceError, "dimension product"):
            infer(payload([], [], [], batch=(1 << 64) - 1, features=2))

    def test_array_shapes(self):
        for field in ("inputs", "weights", "biases"):
            for value in ([], [1, 2], None, "1"):
                task = payload([1], [1], [0])
                task[field] = value
                with self.subTest(field=field, value=value), self.assertRaises(InferenceError):
                    infer(task)

    def test_missing_or_extra_fields(self):
        task = payload([1], [1], [0])
        for field in tuple(task):
            bad = dict(task)
            del bad[field]
            with self.subTest(field=field), self.assertRaises(InferenceError):
                infer(bad)
        with self.assertRaises(InferenceError):
            infer(dict(task, taskId="not-a-network-task"))

    def test_input_is_not_mutated(self):
        task = payload([1], [1], [0])
        before = copy.deepcopy(task)
        infer(task)
        self.assertEqual(task, before)

    def test_strict_json_and_windows_bom(self):
        raw = b'{"batch_size":1,"features":1,"classes":1,"inputs":[1],"weights":[1],"biases":[0]}'
        self.assertEqual(infer(load_payload(b"\xef\xbb\xbf" + raw))["predictions"], [0])
        duplicate = raw.replace(b'"batch_size":1', b'"batch_size":1,"batch_size":2')
        for bad in (duplicate, raw + b"{}", raw.replace(b"[1]", b"[NaN]", 1), b"not json"):
            with self.subTest(raw=bad), self.assertRaises(InferenceError):
                load_payload(bad)


if __name__ == "__main__":
    unittest.main()

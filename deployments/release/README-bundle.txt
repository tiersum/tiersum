TierSum release bundle
=======================

This archive contains:

- tiersum (or tiersum.exe on Windows) — server binary
- third_party/minilm/ — MiniLM ONNX + tokenizer (all-MiniLM-L6-v2)
- third_party/onnxruntime/<platform>/lib — ONNX Runtime CPU library for this bundle
- configs/config.example.yaml — cold_index.embedding paths match this layout (copy to config.yaml)

Quick start:

1. Copy configs/config.example.yaml to configs/config.yaml and set llm.* (API keys, provider, model).
2. From the root of the unpacked directory, run:

     ./tiersum --config configs/config.yaml

   On Windows:

     tiersum.exe --config configs/config.yaml

Third-party licenses: see third_party/ (e.g. LICENSE.onnxruntime) when present.

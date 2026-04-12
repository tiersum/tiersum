TierSum Linux release bundle
=============================

This archive contains:

- tiersum — server binary (CGO, SQLite + gojieba + cold index)
- third_party/onnxruntime/<arch>/lib — ONNX Runtime CPU shared library for this architecture
- third_party/minilm/ — MiniLM ONNX model + tokenizer (sentence-transformers/all-MiniLM-L6-v2)
- configs/config.example.yaml — paths for cold_index.embedding match this bundle layout

Quick start:

1. Copy configs/config.example.yaml to configs/config.yaml and set llm.* (API keys, model, etc.).
2. From the root of the unpacked directory, run:

     ./tiersum --config configs/config.yaml

Third-party notices: ONNX Runtime (Microsoft) and MiniLM weights (Hugging Face) are subject to their
respective licenses; see files under third_party/ when present (e.g. LICENSE.onnxruntime).

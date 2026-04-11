# all-MiniLM-L6-v2 (ONNX + tokenizer, optional)

TierSum loads **sentence-transformers/all-MiniLM-L6-v2** from disk (HF `onnx/model.onnx` + `tokenizer.json`); there is no `go:embed` model bundle in the binary.

## Fetch

From the repository root:

```bash
make fetch-minilm
# or
chmod +x scripts/fetch-minilm.sh && ./scripts/fetch-minilm.sh
```

This downloads:

- `model.onnx` — HF `onnx/model.onnx` (output `last_hidden_state`; TierSum applies masked mean pool + L2 norm, same idea as sentence-transformers).
- `tokenizer.json` — HF tokenizer config.

Override the Hugging Face repo with `MINILM_HF_REPO=owner/name` if you mirror the model.

## Configure

In `configs/config.yaml`:

```yaml
memory_index:
  embedding:
    provider: auto
    onnx_runtime_path: third_party/onnxruntime/darwin_arm64/lib/libonnxruntime.dylib   # example
    minilm_model_path: third_party/minilm/model.onnx
    # minilm_tokenizer_path: ""   # default: tokenizer.json next to model.onnx
```

`minilm_model_path` must point at the ONNX file (and tokenizer next to it unless overridden). If MiniLM cannot load, `provider: auto` falls back to simple hash embeddings.

## Git

`model.onnx` is large; it is **gitignored**. Run `make fetch-minilm` locally or in CI before build.
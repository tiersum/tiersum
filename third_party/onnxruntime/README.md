# ONNX Runtime (local, optional)

TierSum’s **MiniLM** cold-document embeddings load **ONNX Runtime** as a shared library. You can vendor it here instead of installing via the OS package manager.

## Fetch

From the repository root:

```bash
make fetch-onnxruntime
# or
./scripts/fetch-onnxruntime.sh        # current machine only
./scripts/fetch-onnxruntime.sh all     # linux_amd64, linux_arm64, darwin_amd64, darwin_arm64
```

This downloads Microsoft’s CPU build (default version in `scripts/fetch-onnxruntime.sh`, currently aligned with `deployments/docker/Dockerfile`). Libraries land under `third_party/onnxruntime/<platform>/lib/`.

## Configure

In `configs/config.yaml`, point at the library for your platform (run from repo root or use an absolute path):


| Platform    | Example `onnx_runtime_path`                                     |
| ----------- | --------------------------------------------------------------- |
| macOS arm64 | `third_party/onnxruntime/darwin_arm64/lib/libonnxruntime.dylib` |
| macOS x64   | `third_party/onnxruntime/darwin_amd64/lib/libonnxruntime.dylib` |
| Linux x64   | `third_party/onnxruntime/linux_amd64/lib/libonnxruntime.so`     |
| Linux arm64 | `third_party/onnxruntime/linux_arm64/lib/libonnxruntime.so`     |


Alternatively set `ONNXRUNTIME_LIB_PATH` to the same file path.

`memory_index.embedding.provider: auto` will use MiniLM when this library loads; otherwise it falls back to simple embeddings.

## Git

Runtime binaries are **gitignored** (large). Each developer or CI job runs `make fetch-onnxruntime` once (or cache `third_party/onnxruntime/`).

## Docker

The `deployments/docker/Dockerfile` image installs ONNX Runtime under `/usr/local/lib` and downloads MiniLM into `/app/third_party/minilm/` during build; it does not use host `third_party/onnxruntime/`. This README targets local `make run` / bare-metal workflows.

## MiniLM model weights

ONNX Runtime loads the network; **MiniLM ONNX + tokenizer** are separate files. Use `make fetch-minilm` → `third_party/minilm/` (see that directory’s README). Nothing is `go:embed`’d for the model in the Go binary.
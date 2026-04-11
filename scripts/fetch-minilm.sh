#!/usr/bin/env bash
# Downloads all-MiniLM-L6-v2 ONNX + tokenizer from Hugging Face into third_party/minilm/
# (reproducible; avoids Go module Git LFS pointer issues for model.onnx).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${ROOT}/third_party/minilm"
mkdir -p "${DEST}"

BASE="${MINILM_HF_REPO:-sentence-transformers/all-MiniLM-L6-v2}"
RESOLVE="https://huggingface.co/${BASE}/resolve/main"

echo "Repo: ${BASE}"
echo "Destination: ${DEST}"

curl_minilm() {
  local url="$1"
  local out="$2"
  echo "Downloading $(basename "$out") ..."
  curl -fsSL -o "$out" -H "Accept: application/octet-stream" "$url"
}

# ONNX with inputs input_ids / attention_mask / token_type_ids and output last_hidden_state (mean-pooled in TierSum).
curl_minilm "${RESOLVE}/onnx/model.onnx" "${DEST}/model.onnx"
curl_minilm "${RESOLVE}/tokenizer.json" "${DEST}/tokenizer.json"

ls -lh "${DEST}/model.onnx" "${DEST}/tokenizer.json"
echo "Done. Set memory_index.embedding.minilm_model_path (see third_party/minilm/README.md)."

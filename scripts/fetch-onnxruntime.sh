#!/usr/bin/env bash
# Downloads ONNX Runtime CPU shared libraries into third_party/onnxruntime/ so TierSum can use
# MiniLM without a system-wide install. Run from any directory.
set -euo pipefail

VERSION="${ONNXRUNTIME_VERSION:-1.19.2}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${ROOT}/third_party/onnxruntime"
MODE="${1:-host}"

mkdir -p "${DEST}"

download_extract() {
  local name="$1"
  local url="$2"
  local out_subdir="$3"
  local tmp
  tmp="$(mktemp -d)"
  echo "Fetching ${name} ..."
  curl -fsSL -o "${tmp}/onnx.tgz" "${url}"
  tar xf "${tmp}/onnx.tgz" -C "${tmp}"
  local inner
  inner="$(find "${tmp}" -maxdepth 1 -type d -name 'onnxruntime-*' | head -1)"
  if [[ ! -d "${inner}/lib" ]]; then
    rm -rf "${tmp}"
    echo "unexpected archive layout for ${name}" >&2
    exit 1
  fi
  local target="${DEST}/${out_subdir}"
  rm -rf "${target}"
  mkdir -p "${target}"
  cp -R "${inner}/lib" "${target}/"
  if [[ -f "${inner}/LICENSE" ]]; then
    cp "${inner}/LICENSE" "${target}/LICENSE.onnxruntime"
  fi
  rm -rf "${tmp}"
  echo "  -> ${target}/lib"
}

if [[ "${MODE}" == "all" ]]; then
  download_extract "linux x64" \
    "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-linux-x64-${VERSION}.tgz" \
    "linux_amd64"
  download_extract "linux aarch64" \
    "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-linux-aarch64-${VERSION}.tgz" \
    "linux_arm64"
  download_extract "macOS x86_64" \
    "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-osx-x86_64-${VERSION}.tgz" \
    "darwin_amd64"
  download_extract "macOS arm64" \
    "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-osx-arm64-${VERSION}.tgz" \
    "darwin_arm64"
elif [[ "${MODE}" == "host" ]]; then
  case "$(uname -s)/$(uname -m)" in
    Linux/x86_64)
      download_extract "linux x64" \
        "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-linux-x64-${VERSION}.tgz" \
        "linux_amd64"
      ;;
    Linux/aarch64)
      download_extract "linux aarch64" \
        "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-linux-aarch64-${VERSION}.tgz" \
        "linux_arm64"
      ;;
    Darwin/arm64)
      download_extract "macOS arm64" \
        "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-osx-arm64-${VERSION}.tgz" \
        "darwin_arm64"
      ;;
    Darwin/x86_64)
      download_extract "macOS x86_64" \
        "https://github.com/microsoft/onnxruntime/releases/download/v${VERSION}/onnxruntime-osx-x86_64-${VERSION}.tgz" \
        "darwin_amd64"
      ;;
    *)
      echo "Unsupported host $(uname -s)/$(uname -m). Use: $0 all" >&2
      exit 1
      ;;
  esac
else
  echo "Usage: $0 [host|all]" >&2
  echo "  host  (default) — current OS/arch only" >&2
  echo "  all   — linux_amd64, linux_arm64, darwin_amd64, darwin_arm64" >&2
  exit 1
fi

echo "${VERSION}" > "${DEST}/VERSION"
echo "Done. Set memory_index.embedding.onnx_runtime_path (see third_party/onnxruntime/README.md)."

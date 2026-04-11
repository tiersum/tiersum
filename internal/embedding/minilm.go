package embedding

import (
	"context"
	"fmt"
	"strings"
)

// MiniLM runs all-MiniLM-L6-v2 (384-dim) via ONNX Runtime using on-disk HF ONNX + tokenizer
// (mean pool over last_hidden_state). Run `make fetch-minilm` and set memory_index.embedding.minilm_model_path.
type MiniLM struct {
	hf *hfMiniLM
}

// NewMiniLM loads MiniLM from disk. tokenizerPath may be empty to use tokenizer.json next to modelPath.
func NewMiniLM(runtimePath, modelPath, tokenizerPath string) (*MiniLM, error) {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return nil, fmt.Errorf("minilm_model_path is required (run `make fetch-minilm` and set memory_index.embedding.minilm_model_path)")
	}
	hf, err := newHFMiniLMFromFiles(modelPath, tokenizerPath, runtimePath)
	if err != nil {
		return nil, err
	}
	return &MiniLM{hf: hf}, nil
}

// Embed implements TextEmbedder.
func (e *MiniLM) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.hf.embed(ctx, text)
}

// Close implements TextEmbedder.
func (e *MiniLM) Close() error {
	if e.hf == nil {
		return nil
	}
	return e.hf.close()
}

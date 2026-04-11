package embedding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// resolveEmbeddingFilePath resolves a config path against the process working directory at startup.
func resolveEmbeddingFilePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(wd, p))
}

// NewFromViper builds a TextEmbedder from memory_index.embedding.* settings.
//
// provider:
//   - "" or "auto" (default): try all-MiniLM-L6-v2 + ONNX Runtime; on init failure use simple hash embedding.
//   - "simple": legacy hash/n-gram projection only (no ONNX).
//   - "minilm" or "all-minilm-l6-v2": MiniLM only; startup error if ONNX/model cannot load.
func NewFromViper(logger *zap.Logger) (TextEmbedder, error) {
	p := strings.ToLower(strings.TrimSpace(viper.GetString("memory_index.embedding.provider")))
	rt := resolveEmbeddingFilePath(viper.GetString("memory_index.embedding.onnx_runtime_path"))
	modelPath := resolveEmbeddingFilePath(viper.GetString("memory_index.embedding.minilm_model_path"))
	tokPath := resolveEmbeddingFilePath(viper.GetString("memory_index.embedding.minilm_tokenizer_path"))

	tryMiniLM := func() (TextEmbedder, error) {
		return NewMiniLM(rt, modelPath, tokPath)
	}

	switch {
	case p == "simple":
		return NewSimple(), nil
	case p == "minilm" || p == "all-minilm-l6-v2":
		e, err := tryMiniLM()
		if err != nil {
			return nil, fmt.Errorf("minilm embedder: %w", err)
		}
		if logger != nil {
			logMiniLMStartup(logger, rt, modelPath, tokPath)
		}
		return e, nil
	case p == "" || p == "auto":
		e, err := tryMiniLM()
		if err != nil {
			if logger != nil {
				logger.Info("MiniLM embedder unavailable, using simple cold embeddings",
					zap.String("onnx_runtime_path", rt),
					zap.String("minilm_model_path", modelPath),
					zap.Error(err))
			}
			return NewSimple(), nil
		}
		if logger != nil {
			logMiniLMStartup(logger, rt, modelPath, tokPath)
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unknown memory_index.embedding.provider %q (use auto, simple, or minilm)", p)
	}
}

func logMiniLMStartup(logger *zap.Logger, onnxRT, modelPath, tokPath string) {
	logger.Info("cold document embeddings: MiniLM-L6-v2 (ONNX, HF onnx + mean pool)",
		zap.String("onnx_runtime_path", onnxRT),
		zap.String("minilm_model_path", modelPath),
		zap.String("minilm_tokenizer_path", tokPath),
	)
}

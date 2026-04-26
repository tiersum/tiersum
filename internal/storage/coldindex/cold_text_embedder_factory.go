package coldindex

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

// NewTextEmbedderFromViper builds an IColdTextEmbedder from cold_index.embedding.* viper keys.
//
// provider:
//   - "" or "auto" (default): try all-MiniLM-L6-v2 + ONNX Runtime; startup error if unavailable.
//   - "minilm" or "all-minilm-l6-v2": MiniLM only; startup error if ONNX/model cannot load.
func NewTextEmbedderFromViper(logger *zap.Logger) (IColdTextEmbedder, error) {
	p := strings.ToLower(strings.TrimSpace(viper.GetString("cold_index.embedding.provider")))
	rt := resolveEmbeddingFilePath(viper.GetString("cold_index.embedding.onnx_runtime_path"))
	modelPath := resolveEmbeddingFilePath(viper.GetString("cold_index.embedding.minilm_model_path"))
	tokPath := resolveEmbeddingFilePath(viper.GetString("cold_index.embedding.minilm_tokenizer_path"))

	tryMiniLM := func() (IColdTextEmbedder, error) {
		return NewMiniLM(rt, modelPath, tokPath)
	}

	switch {
	case p == "minilm" || p == "all-minilm-l6-v2" || p == "" || p == "auto":
		e, err := tryMiniLM()
		if err != nil {
			return nil, fmt.Errorf("minilm embedder: %w", err)
		}
		if logger != nil {
			logMiniLMStartup(logger, rt, modelPath, tokPath)
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unknown cold_index.embedding.provider %q (use auto or minilm)", p)
	}
}

func logMiniLMStartup(logger *zap.Logger, onnxRT, modelPath, tokPath string) {
	logger.Info("cold document embeddings: MiniLM-L6-v2 (ONNX, HF onnx + mean pool)",
		zap.String("onnx_runtime_path", onnxRT),
		zap.String("minilm_model_path", modelPath),
		zap.String("minilm_tokenizer_path", tokPath),
	)
}

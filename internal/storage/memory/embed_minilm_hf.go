package memory

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"

	"github.com/tiersum/tiersum/pkg/types"
)

const minilmHiddenSize = 384

// hfMiniLM runs Hugging Face–style all-MiniLM-L6-v2 ONNX (output: last_hidden_state) with
// mean pooling + L2 normalization, matching sentence-transformers inference.
type hfMiniLM struct {
	tk      tokenizer.Tokenizer
	session *ort.DynamicAdvancedSession
}

func newHFMiniLMFromFiles(onnxPath, tokenizerPath, runtimePath string) (*hfMiniLM, error) {
	onnxPath = strings.TrimSpace(onnxPath)
	if onnxPath == "" {
		return nil, fmt.Errorf("onnx model path is empty")
	}
	st, err := os.Stat(onnxPath)
	if err != nil {
		return nil, fmt.Errorf("stat onnx model: %w", err)
	}
	if st.Size() < 1<<20 {
		return nil, fmt.Errorf("onnx model at %q is too small (%d bytes); expected full MiniLM ONNX (~80MB+), not a Git LFS pointer", onnxPath, st.Size())
	}
	onnxBytes, err := os.ReadFile(onnxPath)
	if err != nil {
		return nil, fmt.Errorf("read onnx model: %w", err)
	}
	if len(onnxBytes) >= 64 && bytes.HasPrefix(onnxBytes, []byte("version https://git-lfs")) {
		return nil, fmt.Errorf("onnx file at %q looks like a Git LFS pointer, not model weights; run scripts/fetch-minilm.sh", onnxPath)
	}

	tokenizerPath = strings.TrimSpace(tokenizerPath)
	if tokenizerPath == "" {
		tokenizerPath = filepath.Join(filepath.Dir(onnxPath), "tokenizer.json")
	}
	tokBytes, err := os.ReadFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("read tokenizer: %w", err)
	}
	tkPtr, err := pretrained.FromReader(bytes.NewReader(tokBytes))
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	if strings.TrimSpace(runtimePath) != "" {
		ort.SetSharedLibraryPath(runtimePath)
	} else if path, ok := os.LookupEnv("ONNXRUNTIME_LIB_PATH"); ok {
		ort.SetSharedLibraryPath(path)
	}
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("onnx runtime init: %w", err)
	}

	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"}
	session, err := ort.NewDynamicAdvancedSessionWithONNXData(onnxBytes, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("onnx session: %w", err)
	}

	return &hfMiniLM{tk: *tkPtr, session: session}, nil
}

func (m *hfMiniLM) embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	out, err := m.embedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty embedding batch")
	}
	if len(out[0]) != types.ColdEmbeddingVectorDimension {
		return nil, fmt.Errorf("minilm embedding dim %d, want %d", len(out[0]), types.ColdEmbeddingVectorDimension)
	}
	return out[0], nil
}

func (m *hfMiniLM) embedBatch(sentences []string) ([][]float32, error) {
	if len(sentences) == 0 {
		return nil, nil
	}
	inputBatch := make([]tokenizer.EncodeInput, 0, len(sentences))
	for _, s := range sentences {
		inputBatch = append(inputBatch, tokenizer.NewSingleEncodeInput(tokenizer.NewRawInputSequence(s)))
	}
	encodings, err := m.tk.EncodeBatch(inputBatch, true)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}
	return m.runMeanPool(encodings)
}

func (m *hfMiniLM) runMeanPool(encodings []tokenizer.Encoding) ([][]float32, error) {
	batchSize := len(encodings)
	if batchSize == 0 {
		return nil, nil
	}
	seqLength := len(encodings[0].Ids)
	hiddenSize := minilmHiddenSize

	inputShape := ort.NewShape(int64(batchSize), int64(seqLength))
	inputIdsData := make([]int64, batchSize*seqLength)
	attentionMaskData := make([]int64, batchSize*seqLength)
	tokenTypeIdsData := make([]int64, batchSize*seqLength)
	for b := range batchSize {
		for i, id := range encodings[b].Ids {
			inputIdsData[b*seqLength+i] = int64(id)
		}
		for i, mask := range encodings[b].AttentionMask {
			attentionMaskData[b*seqLength+i] = int64(mask)
		}
		for i, typeID := range encodings[b].TypeIds {
			tokenTypeIdsData[b*seqLength+i] = int64(typeID)
		}
	}

	inputIdsTensor, err := ort.NewTensor(inputShape, inputIdsData)
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor: %w", err)
	}
	defer inputIdsTensor.Destroy()

	attentionMaskTensor, err := ort.NewTensor(inputShape, attentionMaskData)
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor: %w", err)
	}
	defer attentionMaskTensor.Destroy()

	tokenTypeIdsTensor, err := ort.NewTensor(inputShape, tokenTypeIdsData)
	if err != nil {
		return nil, fmt.Errorf("token_type_ids tensor: %w", err)
	}
	defer tokenTypeIdsTensor.Destroy()

	hiddenShape := ort.NewShape(int64(batchSize), int64(seqLength), int64(hiddenSize))
	hiddenTensor, err := ort.NewEmptyTensor[float32](hiddenShape)
	if err != nil {
		return nil, fmt.Errorf("hidden tensor: %w", err)
	}
	defer hiddenTensor.Destroy()

	inputTensors := []ort.Value{inputIdsTensor, attentionMaskTensor, tokenTypeIdsTensor}
	outputTensors := []ort.Value{hiddenTensor}
	if err := m.session.Run(inputTensors, outputTensors); err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}

	flat := hiddenTensor.GetData()
	return meanPoolL2Norm(batchSize, seqLength, hiddenSize, flat, attentionMaskData), nil
}

// meanPoolL2Norm applies masked mean pooling over the sequence dimension and L2-normalizes each row.
func meanPoolL2Norm(batchSize, seqLen, hidden int, hiddenFlat []float32, mask []int64) [][]float32 {
	out := make([][]float32, batchSize)
	for b := 0; b < batchSize; b++ {
		acc := make([]float32, hidden)
		var n float32
		for s := 0; s < seqLen; s++ {
			if mask[b*seqLen+s] == 0 {
				continue
			}
			n++
			base := b*seqLen*hidden + s*hidden
			for h := 0; h < hidden; h++ {
				acc[h] += hiddenFlat[base+h]
			}
		}
		if n < 1e-6 {
			n = 1
		}
		inv := 1 / n
		for h := 0; h < hidden; h++ {
			acc[h] *= inv
		}
		var sumsq float64
		for h := 0; h < hidden; h++ {
			v := float64(acc[h])
			sumsq += v * v
		}
		if sumsq < 1e-12 {
			sumsq = 1
		}
		invn := float32(1 / math.Sqrt(sumsq))
		for h := 0; h < hidden; h++ {
			acc[h] *= invn
		}
		out[b] = acc
	}
	return out
}

func (m *hfMiniLM) close() error {
	if m.session != nil {
		m.session.Destroy()
	}
	return ort.DestroyEnvironment()
}

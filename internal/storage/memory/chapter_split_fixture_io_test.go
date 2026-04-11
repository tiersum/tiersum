package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

type splitFixtureOutput struct {
	SourceFile string           `json:"source_file"`
	DocID      string           `json:"doc_id"`
	Title      string           `json:"doc_title"`
	MaxTokens  int              `json:"max_tokens"`
	Chapters   []chapterOnDisk  `json:"chapters"`
	Meta       splitFixtureMeta `json:"meta"`
}

type chapterOnDisk struct {
	Path     string `json:"path"`
	Text     string `json:"text"`
	TokenEst int    `json:"token_est"`
}

type splitFixtureMeta struct {
	ChapterCount int `json:"chapter_count"`
	CharTotal    int `json:"char_total_input"`
}

func testdataKafkaZkEtcdDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// Markdown fixtures live under testdata/ (numeric *.md); split_io_out/ holds generated IO artifacts.
	return filepath.Join(filepath.Dir(file), "testdata")
}

func testdataSplitOutDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata", "split_io_out")
}

// Committed boundary in/out goldens (no heading vs heading vs nested + sliding split) live under
// testdata/chapter_split_boundaries/; see TestChapterSplitBoundaryGolden.
//
// TestSplitMarkdown_KafkaZkEtcdFixtures_IO reads 10 markdown articles, splits them,
// and writes JSON input/output under testdata/split_io_out/ for offline review.
func TestSplitMarkdown_KafkaZkEtcdFixtures_IO(t *testing.T) {
	inDir := testdataKafkaZkEtcdDir(t)
	outDir := testdataSplitOutDir(t)
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	entries, err := os.ReadDir(inDir)
	require.NoError(t, err)

	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		mdFiles = append(mdFiles, e.Name())
	}
	if len(mdFiles) < 10 {
		t.Skipf("optional: need 10 *.md under %s for IO regeneration (have %d); boundary goldens use testdata/chapter_split_boundaries/", inDir, len(mdFiles))
	}
	sort.Strings(mdFiles)

	maxTok := types.DefaultColdChapterMaxTokens
	for _, name := range mdFiles {
		srcPath := filepath.Join(inDir, name)
		raw, err := os.ReadFile(srcPath)
		require.NoError(t, err, name)

		docID := strings.TrimSuffix(name, filepath.Ext(name))
		title := humanTitleFromFixtureName(name)
		chapters := SplitMarkdown(docID, title, string(raw), maxTok)
		require.NotEmpty(t, chapters, name)

		out := splitFixtureOutput{
			SourceFile: name,
			DocID:      docID,
			Title:      title,
			MaxTokens:  maxTok,
			Meta: splitFixtureMeta{
				ChapterCount: len(chapters),
				CharTotal:    len(raw),
			},
		}
		for _, s := range chapters {
			out.Chapters = append(out.Chapters, chapterOnDisk{
				Path:     s.Path,
				Text:     s.Text,
				TokenEst: EstimateTokens(s.Text),
			})
		}

		stem := strings.TrimSuffix(name, filepath.Ext(name))
		outPath := filepath.Join(outDir, stem+"_split_output.json")
		enc, err := json.MarshalIndent(out, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(outPath, enc, 0o644), outPath)

		require.NoError(t, os.WriteFile(filepath.Join(outDir, stem+"_input.md"), raw, 0o644))
	}

	manifest := map[string]any{
		"fixture_dir": inDir,
		"output_dir":  outDir,
		"max_tokens":  maxTok,
		"files":       mdFiles,
	}
	mb, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "manifest.json"), mb, 0o644))
}

func humanTitleFromFixtureName(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.SplitN(base, "_", 3)
	if len(parts) >= 3 {
		return strings.ReplaceAll(parts[2], "_", " ")
	}
	return base
}

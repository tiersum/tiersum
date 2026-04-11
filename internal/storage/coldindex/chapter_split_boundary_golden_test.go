package coldindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Boundary golden fixtures live under testdata/chapter_split_boundaries/:
//
//	<stem>.input.md  — markdown input
//	<stem>.golden.json — expected SplitMarkdown output (same shape as split fixture IO)
//
// Regenerate goldens after intentional splitter changes:
//
//	UPDATE_CHAPTER_SPLIT_GOLDEN=1 go test ./internal/storage/coldindex/ -run TestChapterSplitBoundaryGolden -count=1
func TestChapterSplitBoundaryGolden(t *testing.T) {
	t.Cleanup(func() { SetColdMarkdownSlidingStrideTokens(0) })
	SetColdMarkdownSlidingStrideTokens(64)

	const maxTok = 128
	dir := boundaryGoldenDir(t)

	cases := []struct {
		stem  string
		docID string
		title string
	}{
		{"no_parent_oversized", "boundary_no_parent", "No Parent Oversized"},
		{"with_parent_oversized", "boundary_with_parent", "With Parent Oversized"},
		{"nested_parent_oversized", "boundary_nested", "Nested Parent Oversized"},
	}

	for _, tc := range cases {
		t.Run(tc.stem, func(t *testing.T) {
			inPath := filepath.Join(dir, tc.stem+".input.md")
			raw, err := os.ReadFile(inPath)
			require.NoError(t, err)

			chapters := SplitMarkdown(tc.docID, tc.title, string(raw), maxTok)
			require.NotEmpty(t, chapters)

			got := buildBoundaryGolden(tc.stem, tc.docID, tc.title, maxTok, raw, chapters)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			require.NoError(t, err)

			goldenPath := filepath.Join(dir, tc.stem+".golden.json")
			if os.Getenv("UPDATE_CHAPTER_SPLIT_GOLDEN") == "1" {
				require.NoError(t, os.WriteFile(goldenPath, gotJSON, 0o644), "write golden %s", goldenPath)
				t.Logf("wrote %s", goldenPath)
				return
			}

			wantRaw, err := os.ReadFile(goldenPath)
			require.NoError(t, err, "missing golden %s (run with UPDATE_CHAPTER_SPLIT_GOLDEN=1)", goldenPath)

			var want, gotObj splitFixtureOutput
			require.NoError(t, json.Unmarshal(wantRaw, &want))
			require.NoError(t, json.Unmarshal(gotJSON, &gotObj))
			require.Equal(t, want.SourceFile, gotObj.SourceFile)
			require.Equal(t, want.DocID, gotObj.DocID)
			require.Equal(t, want.Title, gotObj.Title)
			require.Equal(t, want.MaxTokens, gotObj.MaxTokens)
			require.Equal(t, want.Meta, gotObj.Meta)
			require.Equal(t, len(want.Chapters), len(gotObj.Chapters), "chapter count")
			for i := range want.Chapters {
				require.Equal(t, want.Chapters[i].Path, gotObj.Chapters[i].Path, "path chapter %d", i)
				require.Equal(t, want.Chapters[i].TokenEst, gotObj.Chapters[i].TokenEst, "token_est chapter %d", i)
				require.Equal(t, want.Chapters[i].Text, gotObj.Chapters[i].Text, "text chapter %d", i)
			}
		})
	}
}

func boundaryGoldenDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata", "chapter_split_boundaries")
}

func buildBoundaryGolden(sourceFile, docID, title string, maxTok int, raw []byte, chapters []ColdChapter) splitFixtureOutput {
	out := splitFixtureOutput{
		SourceFile: sourceFile + ".input.md",
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
	return out
}

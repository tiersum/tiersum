package coldindex

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

// Section ## 2 is one cold chapter path; merged body must re-include ### 2.1 / ### 2.2 heading lines (not only prose).
func TestSplitMarkdown_kafkaDiskStructure_section2MergedPreservesHeadings(t *testing.T) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	mdPath := filepath.Join(filepath.Dir(file), "testdata", "split_io_out", "kafka-disk-structure.md")
	raw, err := os.ReadFile(mdPath)
	require.NoError(t, err, "read %s", mdPath)

	const docID = "kafka-disk-structure"
	title := "Kafka 磁盘数据结构详解"
	chapters := SplitMarkdown(docID, title, string(raw), types.DefaultColdChapterMaxTokens)
	require.NotEmpty(t, chapters)

	var sec2 *ColdChapter
	for i := range chapters {
		ch := &chapters[i]
		if !strings.Contains(ch.Path, "2. 日志目录结构") {
			continue
		}
		if strings.Contains(ch.Path, "2.1 分区文件组织") || strings.Contains(ch.Path, "2.2 核心文件说明") {
			continue
		}
		sec2 = ch
		break
	}
	require.NotNil(t, sec2, "expected merged chapter for ## 2. 日志目录结构 (paths: %v)", chapterPaths(chapters))
	assert.Contains(t, sec2.Text, "### 2.1 分区文件组织")
	assert.Contains(t, sec2.Text, "### 2.2 核心文件说明")
	assert.Contains(t, sec2.Text, "topic-a-0")
	assert.Contains(t, sec2.Text, "文件类型")
	assert.Contains(t, sec2.Text, ".log")

	for _, ch := range chapters {
		assert.NotContains(t, ch.Path, "页缓存依赖", "ghost chapter from list item: %s", ch.Path)
	}
}

func chapterPaths(ch []ColdChapter) []string {
	out := make([]string, len(ch))
	for i := range ch {
		out[i] = ch[i].Path
	}
	return out
}

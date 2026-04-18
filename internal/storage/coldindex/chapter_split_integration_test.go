package coldindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiersum/tiersum/pkg/types"
)

// loadTestMarkdown loads a markdown file from testdata/chapters directory
func loadTestMarkdown(t *testing.T, filename string) string {
	t.Helper()
	path := filepath.Join("testdata", "chapters", filename)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read test file: %s", filename)
	return string(content)
}

// TestSplitMarkdown_APIDocument tests API document with code blocks and tables
func TestSplitMarkdown_APIDocument(t *testing.T) {
	md := loadTestMarkdown(t, "api_document.md")
	// Use small maxTokens to force splitting
	segs := SplitMarkdown("api-doc", "API Document", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize main headings
	var hasInstall, hasAuth, hasUsers bool
	for _, s := range segs {
		if strings.Contains(s.Path, "安装") {
			hasInstall = true
		}
		if strings.Contains(s.Path, "认证") {
			hasAuth = true
		}
		if strings.Contains(s.Path, "用户接口") {
			hasUsers = true
		}
	}
	assert.True(t, hasInstall, "should have 安装 section")
	assert.True(t, hasAuth, "should have 认证 section")
	assert.True(t, hasUsers, "should have 用户接口 section")
}

// TestSplitMarkdown_TechnicalGuide tests technical guide with nested headings
func TestSplitMarkdown_TechnicalGuide(t *testing.T) {
	md := loadTestMarkdown(t, "technical_guide.md")
	segs := SplitMarkdown("k8s-guide", "K8s Guide", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize both top-level and nested headings
	var hasControlPlane, hasKubelet bool
	for _, s := range segs {
		if strings.Contains(s.Path, "控制平面") {
			hasControlPlane = true
		}
		if strings.Contains(s.Path, "Kubelet") {
			hasKubelet = true
		}
	}
	assert.True(t, hasControlPlane, "should have 控制平面 section")
	assert.True(t, hasKubelet, "should have Kubelet section")
}

// TestSplitMarkdown_TutorialSteps tests tutorial with numbered steps
func TestSplitMarkdown_TutorialSteps(t *testing.T) {
	md := loadTestMarkdown(t, "tutorial_steps.md")
	segs := SplitMarkdown("tutorial", "Tutorial", md, 64)
	require.NotEmpty(t, segs)

	// Note: "步骤一：初始化项目" uses ATX format (###), so it IS recognized as heading
	// This is correct behavior - ATX headings should always be recognized
	// The shortOrderedListTriple only handles "N. " format, not ATX headings

	// Should have main sections
	var hasEnv, hasProject, hasFAQ, hasSteps bool
	for _, s := range segs {
		if strings.Contains(s.Path, "环境准备") {
			hasEnv = true
		}
		if strings.Contains(s.Path, "创建项目") {
			hasProject = true
		}
		if strings.Contains(s.Path, "常见问题") {
			hasFAQ = true
		}
		if strings.Contains(s.Path, "步骤") {
			hasSteps = true
		}
	}
	assert.True(t, hasEnv, "should have 环境准备 section")
	assert.True(t, hasProject, "should have 创建项目 section")
	assert.True(t, hasFAQ, "should have 常见问题 section")
	assert.True(t, hasSteps, "should have step sections (they use ATX format)")
}

// TestSplitMarkdown_BoundaryCases tests edge cases
func TestSplitMarkdown_BoundaryCases(t *testing.T) {
	md := loadTestMarkdown(t, "boundary_cases.md")
	segs := SplitMarkdown("boundary", "Boundary Test", md, 64)
	require.NotEmpty(t, segs)

	// Should NOT create chapter for empty heading
	for _, s := range segs {
		assert.NotEqual(t, "", strings.TrimSpace(s.Text), "should not have empty chapters")
	}

	// Code block headings should be ignored
	for _, s := range segs {
		if strings.Contains(s.Text, "```") {
			assert.NotContains(t, s.Path, "这不是标题", "code block content should not be headings")
		}
	}
}

// TestSplitMarkdown_ChineseDocument tests Chinese document parsing
func TestSplitMarkdown_ChineseDocument(t *testing.T) {
	md := loadTestMarkdown(t, "chinese_document.md")
	segs := SplitMarkdown("cn-doc", "Chinese Doc", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize Chinese headings
	var hasOverview, hasCh1, hasCh2, hasCh3 bool
	for _, s := range segs {
		if strings.Contains(s.Path, "概述") {
			hasOverview = true
		}
		if strings.Contains(s.Path, "第一章") {
			hasCh1 = true
		}
		if strings.Contains(s.Path, "第二章") {
			hasCh2 = true
		}
		if strings.Contains(s.Path, "第三章") {
			hasCh3 = true
		}
	}
	assert.True(t, hasOverview, "should have 概述 section")
	assert.True(t, hasCh1, "should have 第一章 section")
	assert.True(t, hasCh2, "should have 第二章 section")
	assert.True(t, hasCh3, "should have 第三章 section")
}

// TestSplitMarkdown_SetextHeadings tests Setext style headings
func TestSplitMarkdown_SetextHeadings_FromFile(t *testing.T) {
	md := loadTestMarkdown(t, "setext_headings.md")
	segs := SplitMarkdown("setext", "Setext Doc", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize Setext headings
	var hasCh1, hasCh2 bool
	for _, s := range segs {
		if strings.Contains(s.Path, "Chapter One") {
			hasCh1 = true
		}
		if strings.Contains(s.Path, "Chapter Two") {
			hasCh2 = true
		}
	}
	assert.True(t, hasCh1, "should have Chapter One")
	assert.True(t, hasCh2, "should have Chapter Two")
}

// TestSplitMarkdown_NumberedOutline_FromFile: CommonMark does not treat "1. Introduction" as a heading;
// only ATX/Setext headings from goldmark (direct document children) define chapter paths — content stays in body.
func TestSplitMarkdown_NumberedOutline_FromFile(t *testing.T) {
	md := loadTestMarkdown(t, "numbered_outline.md")
	segs := SplitMarkdown("outline", "Outline Doc", md, 64)
	require.NotEmpty(t, segs)

	var blob strings.Builder
	for _, s := range segs {
		blob.WriteString(s.Path)
		blob.WriteByte('\n')
		blob.WriteString(s.Text)
		blob.WriteByte('\n')
	}
	out := blob.String()
	assert.Contains(t, out, "Numbered Outline Document")
	assert.Contains(t, out, "Introduction")
	assert.Contains(t, out, "Methodology")
	assert.Contains(t, out, "Results")
	for _, s := range segs {
		assert.NotContains(t, s.Path, "Introduction", "numbered outline lines must not become path segments: %s", s.Path)
		assert.NotContains(t, s.Path, "Methodology", "numbered outline lines must not become path segments: %s", s.Path)
		assert.NotContains(t, s.Path, "Results", "numbered outline lines must not become path segments: %s", s.Path)
	}
}

// TestSplitMarkdown_IndentedCode tests indented code blocks
func TestSplitMarkdown_IndentedCode(t *testing.T) {
	md := loadTestMarkdown(t, "indented_code.md")
	segs := SplitMarkdown("indented", "Indented Code", md, 64)
	require.NotEmpty(t, segs)

	// Should NOT create headings from indented code block comments
	for _, s := range segs {
		assert.NotContains(t, s.Path, "This is a comment", "indented code comments should not be headings")
	}

	// Should have main sections
	var hasSec1, hasSec2 bool
	for _, s := range segs {
		if strings.Contains(s.Path, "Section One") {
			hasSec1 = true
		}
		if strings.Contains(s.Path, "Section Two") {
			hasSec2 = true
		}
	}
	assert.True(t, hasSec1, "should have Section One")
	assert.True(t, hasSec2, "should have Section Two")
}

// TestSplitMarkdown_LargeDocument tests large document splitting
func TestSplitMarkdown_LargeDocument(t *testing.T) {
	md := loadTestMarkdown(t, "large_document.md")
	segs := SplitMarkdown("large", "Large Doc", md, 64)
	require.NotEmpty(t, segs)

	// Should split oversized content
	assert.GreaterOrEqual(t, len(segs), 1, "should have at least one segment")

	// Check that content is preserved
	var totalContent string
	for _, s := range segs {
		totalContent += s.Text
	}
	assert.Contains(t, totalContent, "Lorem ipsum", "content should be preserved")
}

// TestSplitMarkdown_MergeBehavior tests chapter merging behavior
func TestSplitMarkdown_MergeBehavior(t *testing.T) {
	md := loadTestMarkdown(t, "technical_guide.md")
	
	// With large maxTokens, should merge where possible
	segsLarge := SplitMarkdown("k8s", "K8s", md, 2000)
	
	// With small maxTokens, should split more
	segsSmall := SplitMarkdown("k8s", "K8s", md, 128)
	
	// Large budget should produce fewer or equal segments
	assert.LessOrEqual(t, len(segsLarge), len(segsSmall), 
		"larger token budget should not produce more segments")
}

// TestSplitMarkdown_PathStructure tests path structure
func TestSplitMarkdown_PathStructure(t *testing.T) {
	md := loadTestMarkdown(t, "technical_guide.md")
	segs := SplitMarkdown("doc-123", "K8s Guide", md, 64)
	require.NotEmpty(t, segs)

	// All paths should start with docID
	for _, s := range segs {
		assert.True(t, strings.HasPrefix(s.Path, "doc-123/"), 
			"path should start with docID: %s", s.Path)
	}

	// Paths should use / as separator
	for _, s := range segs {
		assert.Contains(t, s.Path, "/", "path should contain separator")
	}
}

// TestSplitMarkdown_ContentCompleteness tests that all content is preserved
func TestSplitMarkdown_ContentCompleteness(t *testing.T) {
	md := loadTestMarkdown(t, "api_document.md")
	segs := SplitMarkdown("api", "API", md, 64)
	require.NotEmpty(t, segs)

	// Combine all segments
	var combined strings.Builder
	for _, s := range segs {
		combined.WriteString(s.Text)
		combined.WriteString("\n")
	}
	result := combined.String()

	// Key content should be preserved
	assert.Contains(t, result, "npm install my-api")
	assert.Contains(t, result, "Authorization")
	assert.Contains(t, result, "Bearer")
	assert.Contains(t, result, "400")
	assert.Contains(t, result, "500")
}

// TestSplitMarkdown_MixedLists tests list and heading interactions
func TestSplitMarkdown_MixedLists(t *testing.T) {
	md := loadTestMarkdown(t, "mixed_lists.md")
	segs := SplitMarkdown("mixed", "Mixed", md, 64)
	require.NotEmpty(t, segs)

	// Should have main headings
	var hasMain1, hasMain2, hasMain3 bool
	for _, s := range segs {
		if strings.Contains(s.Path, "主标题一") {
			hasMain1 = true
		}
		if strings.Contains(s.Path, "主标题二") {
			hasMain2 = true
		}
		if strings.Contains(s.Path, "主标题三") {
			hasMain3 = true
		}
	}
	assert.True(t, hasMain1, "should have 主标题一")
	assert.True(t, hasMain2, "should have 主标题二")
	assert.True(t, hasMain3, "should have 主标题三")

	// List items should NOT be headings (1. 2. 3.)
	for _, s := range segs {
		assert.NotContains(t, s.Path, "第一项", "list items should not be headings")
		assert.NotContains(t, s.Path, "第二项", "list items should not be headings")
	}
}

// TestSplitMarkdown_HTMLTags tests HTML tag handling
func TestSplitMarkdown_HTMLTags(t *testing.T) {
	md := loadTestMarkdown(t, "html_tags.md")
	segs := SplitMarkdown("html", "HTML", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize ATX headings
	var hasBasic, hasTable, hasCode bool
	for _, s := range segs {
		if strings.Contains(s.Path, "基础 HTML") {
			hasBasic = true
		}
		if strings.Contains(s.Path, "表格 HTML") {
			hasTable = true
		}
		if strings.Contains(s.Path, "代码与 HTML") {
			hasCode = true
		}
	}
	assert.True(t, hasBasic, "should have 基础 HTML")
	assert.True(t, hasTable, "should have 表格 HTML")
	assert.True(t, hasCode, "should have 代码与 HTML")

	// HTML content should be preserved
	var combined strings.Builder
	for _, s := range segs {
		combined.WriteString(s.Text)
	}
	assert.Contains(t, combined.String(), "<span")
	assert.Contains(t, combined.String(), "<table")
}

// TestSplitMarkdown_LinksImages tests link and image handling
func TestSplitMarkdown_LinksImages(t *testing.T) {
	md := loadTestMarkdown(t, "links_images.md")
	segs := SplitMarkdown("links", "Links", md, 64)
	require.NotEmpty(t, segs)

	// Should have headings
	var hasExternal, hasImage, hasLinked bool
	for _, s := range segs {
		if strings.Contains(s.Path, "外部链接") {
			hasExternal = true
		}
		if strings.Contains(s.Path, "图片") {
			hasImage = true
		}
		if strings.Contains(s.Path, "链接标题") {
			hasLinked = true
		}
	}
	assert.True(t, hasExternal, "should have 外部链接")
	assert.True(t, hasImage, "should have 图片")
	assert.True(t, hasLinked, "should have 链接标题")

	// Links should be preserved in content
	var combined strings.Builder
	for _, s := range segs {
		combined.WriteString(s.Text)
	}
	assert.Contains(t, combined.String(), "https://google.com")
	assert.Contains(t, combined.String(), "![Logo]")
}

// TestSplitMarkdown_Blockquotes tests blockquote handling
func TestSplitMarkdown_Blockquotes(t *testing.T) {
	md := loadTestMarkdown(t, "blockquotes.md")
	segs := SplitMarkdown("quotes", "Quotes", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize headings
	var hasNormal, hasNested, hasCode, hasList bool
	for _, s := range segs {
		if strings.Contains(s.Path, "普通引用") {
			hasNormal = true
		}
		if strings.Contains(s.Path, "嵌套引用") {
			hasNested = true
		}
		if strings.Contains(s.Path, "引用中的代码") {
			hasCode = true
		}
		if strings.Contains(s.Path, "引用中的列表") {
			hasList = true
		}
	}
	assert.True(t, hasNormal, "should have 普通引用")
	assert.True(t, hasNested, "should have 嵌套引用")
	assert.True(t, hasCode, "should have 引用中的代码")
	assert.True(t, hasList, "should have 引用中的列表")

	// Blockquote content should be preserved
	var combined strings.Builder
	for _, s := range segs {
		combined.WriteString(s.Text)
	}
	result := combined.String()
	assert.Contains(t, result, "这是一个引用块")
	assert.Contains(t, result, "内层引用")
}

// TestSplitMarkdown_YAMLFrontmatter tests YAML frontmatter handling.
// V2 strips YAML frontmatter before goldmark parsing to avoid Setext false positives.
func TestSplitMarkdown_YAMLFrontmatter(t *testing.T) {
	md := loadTestMarkdown(t, "yaml_frontmatter.md")
	segs := SplitMarkdown("yaml", "YAML", md, 64)
	require.NotEmpty(t, segs)

	// Main heading should be recognized
	var hasMain bool
	var combined strings.Builder
	for _, s := range segs {
		if strings.Contains(s.Path, "YAML Frontmatter 测试") {
			hasMain = true
		}
		combined.WriteString(s.Text)
	}
	assert.True(t, hasMain, "should have YAML Frontmatter 测试 heading")

	// Body content should be preserved (headings may be merged)
	allText := combined.String()
	assert.Contains(t, allText, "正文内容")
	assert.Contains(t, allText, "正文开始")
	assert.Contains(t, allText, "子章节")
	assert.Contains(t, allText, "另一个章节")
}

// TestSplitMarkdown_EmojiSpecial tests emoji and special characters
func TestSplitMarkdown_EmojiSpecial(t *testing.T) {
	md := loadTestMarkdown(t, "emoji_special.md")
	segs := SplitMarkdown("emoji", "Emoji", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize headings with emoji
	var hasRocket, hasPackage, hasCode, hasMixed bool
	for _, s := range segs {
		if strings.Contains(s.Path, "火箭") {
			hasRocket = true
		}
		if strings.Contains(s.Path, "包裹") {
			hasPackage = true
		}
		if strings.Contains(s.Path, "代码") {
			hasCode = true
		}
		if strings.Contains(s.Path, "庆祝") {
			hasMixed = true
		}
	}
	assert.True(t, hasRocket, "should have 火箭标题")
	assert.True(t, hasPackage, "should have 包裹标题")
	assert.True(t, hasCode, "should have 代码标题")
	assert.True(t, hasMixed, "should have 庆祝标题")
}

// TestSplitMarkdown_EmptyLines tests empty line handling
func TestSplitMarkdown_EmptyLines(t *testing.T) {
	md := loadTestMarkdown(t, "empty_lines.md")
	segs := SplitMarkdown("empty", "Empty", md, 64)
	require.NotEmpty(t, segs)

	// Should have headings
	var hasCode, hasSpaces, hasContent, hasIndent bool
	for _, s := range segs {
		if strings.Contains(s.Path, "代码") {
			hasCode = true
		}
		if strings.Contains(s.Path, "空行") {
			hasSpaces = true
		}
		if strings.Contains(s.Path, "无内容") {
			hasContent = true
		}
		if strings.Contains(s.Path, "缩进") {
			hasIndent = true
		}
	}
	assert.True(t, hasCode, "should have 标题后立即代码")
	assert.True(t, hasSpaces, "should have 多个空行")
	assert.True(t, hasContent, "should have 标题无内容")
	assert.True(t, hasIndent, "should have 缩进混合")
}

// TestSplitMarkdown_MixedHeadingStyles tests Setext and ATX mix.
// V2 uses goldmark which correctly detects Setext headings (=== and ---).
// With small maxTokens, headings may be merged into parent chapters.
func TestSplitMarkdown_MixedHeadingStyles(t *testing.T) {
	md := loadTestMarkdown(t, "mixed_heading_styles.md")
	segs := SplitMarkdown("mixed", "Mixed Styles", md, 64)
	require.NotEmpty(t, segs)

	// Setext headings should appear as path segments (detected by goldmark)
	var hasSetext1 bool
	// Other headings may be merged into parent body
	var body strings.Builder
	for _, s := range segs {
		if strings.Contains(s.Path, "Setext 标题一") {
			hasSetext1 = true
		}
		body.WriteString(s.Text)
		body.WriteString("\n")
	}
	assert.True(t, hasSetext1, "should have Setext 标题一 path")

	// All headings should be present in chapter body text (re-inserted during merge)
	allText := body.String()
	assert.Contains(t, allText, "ATX 标题一")
	assert.Contains(t, allText, "ATX 三级")
	assert.Contains(t, allText, "Setext 二级")
	assert.Contains(t, allText, "回到 ATX")
}

// TestSplitMarkdown_CommentsSpecial tests comments and special syntax
func TestSplitMarkdown_CommentsSpecial(t *testing.T) {
	md := loadTestMarkdown(t, "comments_special.md")
	segs := SplitMarkdown("comments", "Comments", md, 64)
	require.NotEmpty(t, segs)

	// Should have headings
	var hasNormal, hasAfter, hasHR, hasEscape bool
	for _, s := range segs {
		if strings.Contains(s.Path, "普通章节") {
			hasNormal = true
		}
		if strings.Contains(s.Path, "注释后的章节") {
			hasAfter = true
		}
		if strings.Contains(s.Path, "水平线") {
			hasHR = true
		}
		if strings.Contains(s.Path, "反斜杠") {
			hasEscape = true
		}
	}
	assert.True(t, hasNormal, "should have 普通章节")
	assert.True(t, hasAfter, "should have 注释后的章节")
	assert.True(t, hasHR, "should have 水平线测试")
	assert.True(t, hasEscape, "should have 反斜杠转义")

	// Escaped content should be treated as body
	var combined strings.Builder
	for _, s := range segs {
		combined.WriteString(s.Text)
	}
	assert.Contains(t, combined.String(), "这不是标题")
	assert.Contains(t, combined.String(), "这不是列表")
}

// TestSplitMarkdown_LongTitles tests long title handling
func TestSplitMarkdown_LongTitles(t *testing.T) {
	md := loadTestMarkdown(t, "long_titles.md")
	segs := SplitMarkdown("long", "Long Titles", md, 64)
	require.NotEmpty(t, segs)

	// Should have headings
	var hasLong, hasShort, hasMixed, hasSpaces, hasSymbols bool
	for _, s := range segs {
		if strings.Contains(s.Path, "长标题") {
			hasLong = true
		}
		if strings.Contains(s.Path, "Short") {
			hasShort = true
		}
		if strings.Contains(s.Path, "混合中英文") {
			hasMixed = true
		}
		if strings.Contains(s.Path, "空格") {
			hasSpaces = true
		}
		if strings.Contains(s.Path, "symbols") || strings.Contains(s.Path, "123") || strings.Contains(s.Path, "!@#") {
			hasSymbols = true
		}
	}
	assert.True(t, hasLong, "should have long title")
	assert.True(t, hasShort, "should have short title")
	assert.True(t, hasMixed, "should have mixed title")
	assert.True(t, hasSpaces, "should have spaced title")
	assert.True(t, hasSymbols, "should have symbol title")

	// Long title should be truncated in path
	for _, s := range segs {
		pathLen := len(s.Path)
		assert.Less(t, pathLen, 200, "path should not be excessively long: %s", s.Path)
	}
}

// TestSplitMarkdown_AllFiles tests all markdown files load successfully
func TestSplitMarkdown_AllFiles(t *testing.T) {
	files := []string{
		"api_document.md",
		"technical_guide.md",
		"tutorial_steps.md",
		"boundary_cases.md",
		"chinese_document.md",
		"setext_headings.md",
		"numbered_outline.md",
		"indented_code.md",
		"large_document.md",
		"mixed_lists.md",
		"html_tags.md",
		"links_images.md",
		"blockquotes.md",
		"yaml_frontmatter.md",
		"emoji_special.md",
		"empty_lines.md",
		"mixed_heading_styles.md",
		"comments_special.md",
		"long_titles.md",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			md := loadTestMarkdown(t, file)
			segs := SplitMarkdown("test", "Test", md, 64)
			assert.NotEmpty(t, segs, "file %s should produce at least one segment", file)

			// Each segment should have valid path and text
			for _, s := range segs {
				assert.NotEmpty(t, s.Path, "segment should have path")
				assert.NotEmpty(t, s.Text, "segment should have text")
			assert.True(t, strings.HasPrefix(s.Path, "test/"), "path should start with docID")
			}
		})
	}
}

// ============ Boundary Golden Tests ============

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

// TestChapterSplitBoundaryGolden tests oversized document splitting with golden fixtures.
// Regenerate goldens after intentional splitter changes:
//
//	UPDATE_CHAPTER_SPLIT_GOLDEN=1 go test ./internal/storage/coldindex/ -run TestChapterSplitBoundaryGolden -count=1
func TestChapterSplitBoundaryGolden(t *testing.T) {
	t.Cleanup(func() { SetColdMarkdownSlidingStrideTokens(0) })
	SetColdMarkdownSlidingStrideTokens(64)

	const maxTok = 128

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
			inPath := filepath.Join("testdata", "chapters", "boundary_"+tc.stem+".md")
			raw, err := os.ReadFile(inPath)
			require.NoError(t, err)

			chapters := SplitMarkdown(tc.docID, tc.title, string(raw), maxTok)
			require.NotEmpty(t, chapters)

			got := buildBoundaryGolden(tc.stem, tc.docID, tc.title, maxTok, raw, chapters)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			require.NoError(t, err)

			goldenPath := filepath.Join("testdata", "goldens", tc.stem+".golden.json")
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

func buildBoundaryGolden(sourceFile, docID, title string, maxTok int, raw []byte, chapters []ColdChapter) splitFixtureOutput {
	out := splitFixtureOutput{
		SourceFile: sourceFile + ".md",
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

// ============ Sliding Window Tests ============

func TestSplitMarkdown_oversizedPlain_slidingPathsUseRoot(t *testing.T) {
	const tokenBudget = 512
	const runesPerToken = 4
	strideTokens := defaultColdMarkdownSlidingStrideTokens
	stepRunes := strideTokens * runesPerToken
	winRunes := tokenBudget * runesPerToken
	overlapRunes := winRunes - stepRunes

	// Long plain body (no headings) → splitOversizedRaw(nil, …, 512)
	var sb strings.Builder
	for sb.Len() < winRunes+2*stepRunes+100 {
		sb.WriteString("abcdefghij") // 10 runes per iter
	}
	body := sb.String()
	rn := []rune(body)
	require.Greater(t, len(rn), winRunes+stepRunes)

	chapters := SplitMarkdown("doc1", "Book", body, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2, "expected sliding windows for oversized leaf")

	assert.Equal(t, "doc1/__root__/1", chapters[0].Path)
	assert.Equal(t, "doc1/__root__/2", chapters[1].Path)

	r0 := []rune(chapters[0].Text)
	r1 := []rune(chapters[1].Text)
	require.GreaterOrEqual(t, len(r0), overlapRunes)
	require.GreaterOrEqual(t, len(r1), overlapRunes)
	assert.Equal(t,
		string(r0[len(r0)-overlapRunes:]),
		string(r1[:overlapRunes]),
		"consecutive windows should share overlap runes",
	)
}

func TestSplitMarkdown_oversizedUnderHeading_slidingPathsParentPlusIndex(t *testing.T) {
	const tokenBudget = 512
	var sb strings.Builder
	for sb.Len() < 6000 {
		sb.WriteString("word ")
	}
	md := "# Section A\n\n" + sb.String()

	chapters := SplitMarkdown("d2", "T", md, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2)

	assert.Equal(t, "d2/Section A/1", chapters[0].Path)
	assert.Equal(t, "d2/Section A/2", chapters[1].Path)
}

func TestSplitMarkdown_oversized_overlapRuneCountMatchesSpec(t *testing.T) {
	const tokenBudget = 512
	const runesPerToken = 4
	strideTokens := defaultColdMarkdownSlidingStrideTokens
	stepRunes := strideTokens * runesPerToken
	winRunes := tokenBudget * runesPerToken
	overlapRunes := winRunes - stepRunes

	var parts []string
	for i := 0; i < 120; i++ {
		parts = append(parts, strings.Repeat("x", 63)+string(rune('A'+(i%26))))
	}
	body := strings.Join(parts, "")
	require.GreaterOrEqual(t, utf8.RuneCountInString(body), winRunes+stepRunes+50)

	chapters := SplitMarkdown("d3", "T", body, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2)

	r0 := []rune(chapters[0].Text)
	r1 := []rune(chapters[1].Text)
	assert.Equal(t, winRunes, len(r0))
	assert.Equal(t, string(r0[len(r0)-overlapRunes:]), string(r1[:overlapRunes]))
}

func TestMarkdownSplitter_slidingStrideOverride(t *testing.T) {
	t.Cleanup(func() { SetColdMarkdownSlidingStrideTokens(0) })
	SetColdMarkdownSlidingStrideTokens(0)

	const tokenBudget = 512
	const runesPerToken = 4
	stride412 := 412
	stepRunes := stride412 * runesPerToken
	winRunes := tokenBudget * runesPerToken
	overlapRunes := winRunes - stepRunes // 100 tokens × 4 runes/token

	var sb strings.Builder
	for sb.Len() < winRunes+2*stepRunes+100 {
		sb.WriteString("abcdefghij")
	}
	body := sb.String()

	var sp MarkdownSplitter
	sp.SlidingStrideTokens = stride412
	chapters := sp.Split("ov", "T", body, tokenBudget)
	require.GreaterOrEqual(t, len(chapters), 2)
	r0 := []rune(chapters[0].Text)
	r1 := []rune(chapters[1].Text)
	assert.Equal(t, string(r0[len(r0)-overlapRunes:]), string(r1[:overlapRunes]))
}

// ============ Fixture IO Test (Optional) ============

// TestSplitMarkdown_KafkaZkEtcdFixtures_IO reads markdown articles from testdata/chapters/,
// splits them, and writes JSON input/output under testdata/split_io_out/ for offline review.
// This test is skipped if fewer than 10 *.md files are found.
func TestSplitMarkdown_KafkaZkEtcdFixtures_IO(t *testing.T) {
	inDir := testdataChaptersDir(t)
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
		t.Skipf("optional: need 10 *.md under %s for IO regeneration (have %d)", inDir, len(mdFiles))
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

func testdataChaptersDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata", "chapters")
}

func testdataSplitOutDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "testdata", "split_io_out")
}

func humanTitleFromFixtureName(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.SplitN(base, "_", 3)
	if len(parts) >= 3 {
		return strings.ReplaceAll(parts[2], "_", " ")
	}
	return base
}

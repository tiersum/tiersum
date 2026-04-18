package coldindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestSplitMarkdown_NumberedOutline tests numbered outline headings
func TestSplitMarkdown_NumberedOutline_FromFile(t *testing.T) {
	md := loadTestMarkdown(t, "numbered_outline.md")
	segs := SplitMarkdown("outline", "Outline Doc", md, 64)
	require.NotEmpty(t, segs)

	// Should recognize numbered outline headings
	var hasIntro, hasMethod, hasResults bool
	for _, s := range segs {
		if strings.Contains(s.Path, "Introduction") {
			hasIntro = true
		}
		if strings.Contains(s.Path, "Methodology") {
			hasMethod = true
		}
		if strings.Contains(s.Path, "Results") {
			hasResults = true
		}
	}
	assert.True(t, hasIntro, "should have Introduction")
	assert.True(t, hasMethod, "should have Methodology")
	assert.True(t, hasResults, "should have Results")
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

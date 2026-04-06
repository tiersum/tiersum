package types

// HierarchicalQueryRequest 渐进式层级查询请求
type HierarchicalQueryRequest struct {
	Question   string      `json:"question" binding:"required"`                    // 用户查询
	StartTier  SummaryTier `json:"start_tier" binding:"omitempty,oneof=topic document chapter paragraph source"` // 起始层级，默认 topic
	EndTier    SummaryTier `json:"end_tier" binding:"omitempty,oneof=topic document chapter paragraph source"`   // 终止层级，默认 source
	Tags       []string    `json:"tags,omitempty"`                                 // 可选标签过滤
	MaxResults int         `json:"max_results" binding:"omitempty,min=1,max=50"`  // 每层最大返回数，默认 10
}

// HierarchicalQueryResponse 渐进式层级查询响应
type HierarchicalQueryResponse struct {
	Question string                   `json:"question"` // 原始查询
	Levels   []HierarchicalQueryLevel `json:"levels"`   // 各层级结果
}

// HierarchicalQueryLevel 单层级结果
type HierarchicalQueryLevel struct {
	Tier    SummaryTier `json:"tier"`     // 当前层级
	Items   []QueryItem `json:"items"`    // 筛选后的项目
	HasMore bool        `json:"has_more"` // 是否有更多内容可深入
}

// QueryItem 查询结果项（扩展版）
type QueryItem struct {
	ID         string      `json:"id"`                    // 项目ID（文档ID或主题ID）
	Title      string      `json:"title"`                 // 标题
	Content    string      `json:"content"`               // 内容（摘要或原文）
	Tier       SummaryTier `json:"tier"`                  // 层级
	Path       string      `json:"path"`                  // 路径，如：doc_001/第一章/1
	Relevance  float64     `json:"relevance"`             // LLM评估的相关度 0-1
	IsSource   bool        `json:"is_source"`             // 是否已经是原文（true=不能再深入，false=可以深入）
	ChildCount int         `json:"child_count,omitempty"` // 子项数量
}

// DrillDownRequest 深入查询请求
type DrillDownRequest struct {
	DocumentID string      `form:"document_id" binding:"required"`          // 文档ID
	CurrentTier SummaryTier `form:"current_tier" binding:"required"`        // 当前层级
	Path       string      `form:"path" binding:"required"`                 // 当前路径
	Question   string      `form:"question" binding:"required"`             // 原始查询（用于LLM筛选）
}

// SourceQueryRequest 查询原文请求
type SourceQueryRequest struct {
	DocumentID string `form:"document_id" binding:"required"` // 文档ID
	Path       string `form:"path" binding:"required"`        // 路径
}

// SummaryExtended 扩展的摘要结构（包含层级标记）
type SummaryExtended struct {
	Summary
}

// ChapterInfo 章节信息（用于切分）
type ChapterInfo struct {
	Title   string // 章节标题
	Level   int    // 标题级别（##=2, ###=3）
	Content string // 章节内容
	Offset  int    // 在原文中的偏移量
}

// ParagraphInfo 段落信息
type ParagraphInfo struct {
	Content string // 段落内容
	Offset  int    // 在章节中的偏移量
}

// LLMFilterResult LLM筛选结果
type LLMFilterResult struct {
	ID          string  `json:"id"`           // 项目ID或路径
	Relevance   float64 `json:"relevance"`    // 相关度评分 0-1
	Explanation string  `json:"explanation"`  // 相关性解释（可选）
}

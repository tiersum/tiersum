package types

// LLMFilterResult LLM筛选结果
type LLMFilterResult struct {
	ID          string  `json:"id"`          // 项目ID或路径
	Relevance   float64 `json:"relevance"`   // 相关度评分 0-1
	Explanation string  `json:"explanation"` // 相关性解释（可选）
}

// ProgressiveQueryRequest 新的渐进式查询请求（基于两级标签）
type ProgressiveQueryRequest struct {
	Question   string `json:"question" binding:"required"`         // 用户查询
	MaxResults int    `json:"max_results" binding:"omitempty,min=1,max=100"` // 文档查询数量，默认100
}

// ProgressiveQueryResponse 新的渐进式查询响应
type ProgressiveQueryResponse struct {
	Question string                  `json:"question"`
	Steps    []ProgressiveQueryStep  `json:"steps"`
	Results  []QueryItem             `json:"results"`
}

// ProgressiveQueryStep 查询步骤结果
type ProgressiveQueryStep struct {
	Step      string   `json:"step"`       // 步骤名称：L1_tags, L2_tags, documents, chapters, source
	Input     interface{} `json:"input"`   // 输入数据
	Output    interface{} `json:"output"`  // 输出数据
	Duration  int64    `json:"duration_ms"` // 耗时
}

// QueryItem 查询结果项（扩展版）
type QueryItem struct {
	ID         string      `json:"id"`                    // 项目ID（文档ID）
	Title      string      `json:"title"`                 // 标题
	Content    string      `json:"content"`               // 内容（摘要或原文）
	Tier       SummaryTier `json:"tier"`                  // 层级
	Path       string      `json:"path"`                  // 路径，如：doc_id/chapter_title
	Relevance  float64     `json:"relevance"`             // LLM评估的相关度 0-1
	IsSource   bool        `json:"is_source"`             // 是否已经是原文（true=不能再深入，false=可以深入）
	ChildCount int         `json:"child_count,omitempty"` // 子项数量
}

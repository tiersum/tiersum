package types

// HierarchicalQueryRequest 渐进式层级查询请求
type HierarchicalQueryRequest struct {
	Question   string      `json:"question" binding:"required"`                                      // 用户查询
	StartTier  SummaryTier `json:"start_tier" binding:"omitempty,oneof=document chapter source"`     // 起始层级，默认 document
	EndTier    SummaryTier `json:"end_tier" binding:"omitempty,oneof=document chapter source"`       // 终止层级，默认 source
	Tags       []string    `json:"tags,omitempty"`                                                   // 可选标签过滤
	MaxResults int         `json:"max_results" binding:"omitempty,min=1,max=50"`                    // 每层最大返回数，默认 10
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
	ID         string      `json:"id"`                    // 项目ID（文档ID）
	Title      string      `json:"title"`                 // 标题
	Content    string      `json:"content"`               // 内容（摘要或原文）
	Tier       SummaryTier `json:"tier"`                  // 层级
	Path       string      `json:"path"`                  // 路径，如：doc_id/chapter_title
	Relevance  float64     `json:"relevance"`             // LLM评估的相关度 0-1
	IsSource   bool        `json:"is_source"`             // 是否已经是原文（true=不能再深入，false=可以深入）
	ChildCount int         `json:"child_count,omitempty"` // 子项数量
}

// DrillDownRequest 深入查询请求
type DrillDownRequest struct {
	DocumentID  string      `form:"document_id" binding:"required"`   // 文档ID
	CurrentTier SummaryTier `form:"current_tier" binding:"required"`  // 当前层级
	Path        string      `form:"path" binding:"required"`          // 当前路径
	Question    string      `form:"question" binding:"required"`      // 原始查询（用于LLM筛选）
}

// SourceQueryRequest 查询原文请求
type SourceQueryRequest struct {
	DocumentID string `form:"document_id" binding:"required"` // 文档ID
	Path       string `form:"path" binding:"required"`        // 路径
}

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

// TagGroupFilterRequest 标签聚类过滤请求
type TagGroupFilterRequest struct {
	Question string `json:"question" binding:"required"`
}

// TagGroupFilterResponse 标签聚类过滤响应
type TagGroupFilterResponse struct {
	Question    string            `json:"question"`
	L1Clusters  []TagGroup      `json:"l1_clusters"`  // 一级聚类（用于展示）
	L2Tags      []TagFilterResult `json:"l2_tags"`      // 二级标签过滤结果
}

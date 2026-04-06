// Package api implements API layer
// Includes REST API and MCP protocol handlers
package api

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// MCPServer handles MCP protocol
type MCPServer struct {
	docService   service.IDocumentService
	queryService service.IQueryService
	topicService service.ITopicService
	logger       *zap.Logger
	mcp          *mcpserver.MCPServer
}

// NewMCPServer creates a new MCP server
func NewMCPServer(docService service.IDocumentService, queryService service.IQueryService, topicService service.ITopicService, logger *zap.Logger) *MCPServer {
	s := &MCPServer{
		docService:   docService,
		queryService: queryService,
		topicService: topicService,
		logger:       logger,
	}

	// Create MCP server
	s.mcp = mcpserver.NewMCPServer(
		"tiersum",
		"1.0.0",
		mcpserver.WithResourceCapabilities(true, true),
	)

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() {
	// Query tool - delegates to queryService (Scheme 1: internal recursive)
	queryTool := mcp.NewTool("tiersum_query",
		mcp.WithDescription("Query knowledge base with hierarchical precision. Uses internal LLM-driven progressive filtering."),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to query"),
		),
		mcp.WithString("depth",
			mcp.Description("Query depth: topic, document, chapter, paragraph, or source"),
			mcp.Enum("topic", "document", "chapter", "paragraph", "source"),
		),
	)
	s.mcp.AddTool(queryTool, s.handleQuery)

	// Hierarchical Query tool (Scheme 1 full)
	hierarchicalQueryTool := mcp.NewTool("tiersum_hierarchical_query",
		mcp.WithDescription("Perform full hierarchical query from topic to source level with LLM filtering at each level"),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The question to query"),
		),
		mcp.WithString("start_tier",
			mcp.Description("Start tier: topic (default), document, chapter, paragraph"),
			mcp.Enum("topic", "document", "chapter", "paragraph"),
		),
		mcp.WithString("end_tier",
			mcp.Description("End tier: topic, document, chapter, paragraph, source (default)"),
			mcp.Enum("topic", "document", "chapter", "paragraph", "source"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum results per level (default: 10)"),
		),
	)
	s.mcp.AddTool(hierarchicalQueryTool, s.handleHierarchicalQuery)

	// GetDocument tool - delegates to docService
	getDocTool := mcp.NewTool("tiersum_get_document",
		mcp.WithDescription("Retrieve a document by ID"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID to retrieve"),
		),
	)
	s.mcp.AddTool(getDocTool, s.handleGetDocument)

	// ListTopics tool - lists all topics
	listTopicsTool := mcp.NewTool("tiersum_list_topics",
		mcp.WithDescription("List all topic summaries"),
	)
	s.mcp.AddTool(listTopicsTool, s.handleListTopics)

	// GetTopic tool - retrieves a topic by ID
	getTopicTool := mcp.NewTool("tiersum_get_topic",
		mcp.WithDescription("Retrieve a topic summary by ID"),
		mcp.WithString("topic_id",
			mcp.Required(),
			mcp.Description("The topic ID to retrieve"),
		),
	)
	s.mcp.AddTool(getTopicTool, s.handleGetTopic)

	// === Scheme 2: Agent-driven progressive discovery tools ===

	// GetTopicDocuments tool - get documents under a topic
	getTopicDocsTool := mcp.NewTool("tiersum_get_topic_documents",
		mcp.WithDescription("Get all documents associated with a topic (Scheme 2: Agent-driven discovery)"),
		mcp.WithString("topic_id",
			mcp.Required(),
			mcp.Description("The topic ID to get documents from"),
		),
	)
	s.mcp.AddTool(getTopicDocsTool, s.handleGetTopicDocuments)

	// GetDocumentChapters tool - get chapters of a document
	getDocChaptersTool := mcp.NewTool("tiersum_get_document_chapters",
		mcp.WithDescription("Get all chapters/sections of a document (Scheme 2: Agent-driven discovery)"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID to get chapters from"),
		),
	)
	s.mcp.AddTool(getDocChaptersTool, s.handleGetDocumentChapters)

	// GetChapterParagraphs tool - get paragraphs under a chapter
	getChapterParasTool := mcp.NewTool("tiersum_get_chapter_paragraphs",
		mcp.WithDescription("Get all paragraphs under a chapter (Scheme 2: Agent-driven discovery)"),
		mcp.WithString("chapter_path",
			mcp.Required(),
			mcp.Description("The chapter path (e.g., 'doc_001/Introduction')"),
		),
	)
	s.mcp.AddTool(getChapterParasTool, s.handleGetChapterParagraphs)

	// DrillDown tool - progressive drill-down with LLM filtering
	drillDownTool := mcp.NewTool("tiersum_drill_down",
		mcp.WithDescription("Drill down to the next level with LLM filtering (Scheme 2: Agent-driven discovery). Use this to progressively narrow down content based on your query."),
		mcp.WithString("current_tier",
			mcp.Required(),
			mcp.Description("Current tier level"),
			mcp.Enum("topic", "document", "chapter", "paragraph"),
		),
		mcp.WithString("parent_id",
			mcp.Required(),
			mcp.Description("ID of the current item (topic_id, document_id, or chapter_path)"),
		),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("The query/question to filter items at next level"),
		),
	)
	s.mcp.AddTool(drillDownTool, s.handleDrillDown)

	// GetSource tool - retrieve original source content
	getSourceTool := mcp.NewTool("tiersum_get_source",
		mcp.WithDescription("Retrieve the original source content for a specific path"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("The document ID"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("The path to retrieve source for (e.g., 'doc_001/Chapter1/paragraph_1')"),
		),
	)
	s.mcp.AddTool(getSourceTool, s.handleGetSource)
}

// GetMCPServer returns the underlying MCP server for SSE handling
func (s *MCPServer) GetMCPServer() *mcpserver.MCPServer {
	return s.mcp
}

// SSEHandler returns the SSE handler for MCP
func (s *MCPServer) SSEHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.logger.Info("MCP SSE connection established")
		// TODO: Use mcpserver.NewSSEServer(s.mcp, baseURL) for full implementation
		c.String(200, "MCP SSE endpoint - not fully implemented")
	}
}

// handleQuery handles the tiersum_query tool
func (s *MCPServer) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, ok := request.Params.Arguments["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	depthStr, _ := request.Params.Arguments["depth"].(string)
	if depthStr == "" {
		depthStr = "chapter"
	}

	// Parse depth string to SummaryTier
	depth := types.SummaryTier(depthStr)
	if !isValidDepth(depth) {
		return nil, fmt.Errorf("invalid depth: %s", depthStr)
	}

	s.logger.Info("MCP query", zap.String("question", question), zap.String("depth", string(depth)))

	// Call query service
	results, err := s.queryService.Query(ctx, question, depth)
	if err != nil {
		s.logger.Error("query failed", zap.Error(err))
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Format results
	resultText := formatQueryResults(question, depth, results)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetDocument handles the tiersum_get_document tool
func (s *MCPServer) handleGetDocument(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docID, ok := request.Params.Arguments["document_id"].(string)
	if !ok || docID == "" {
		return nil, fmt.Errorf("document_id is required")
	}

	s.logger.Info("MCP get document", zap.String("document_id", docID))

	// Call document service
	doc, err := s.docService.Get(ctx, docID)
	if err != nil {
		s.logger.Error("failed to get document", zap.String("id", docID), zap.Error(err))
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	if doc == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Document not found: %s", docID)), nil
	}

	// Format document info
	resultText := formatDocument(doc)
	return mcp.NewToolResultText(resultText), nil
}

// handleListTopics handles the tiersum_list_topics tool
func (s *MCPServer) handleListTopics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.Info("MCP list topics")

	topics, err := s.topicService.ListTopics(ctx)
	if err != nil {
		s.logger.Error("failed to list topics", zap.Error(err))
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	resultText := formatTopicList(topics)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetTopic handles the tiersum_get_topic tool
func (s *MCPServer) handleGetTopic(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topicID, ok := request.Params.Arguments["topic_id"].(string)
	if !ok || topicID == "" {
		return nil, fmt.Errorf("topic_id is required")
	}

	s.logger.Info("MCP get topic", zap.String("topic_id", topicID))

	topic, err := s.topicService.GetTopic(ctx, topicID)
	if err != nil {
		s.logger.Error("failed to get topic", zap.String("id", topicID), zap.Error(err))
		return nil, fmt.Errorf("failed to get topic: %w", err)
	}

	if topic == nil {
		return mcp.NewToolResultText(fmt.Sprintf("Topic not found: %s", topicID)), nil
	}

	resultText := formatTopic(topic)
	return mcp.NewToolResultText(resultText), nil
}

// isValidDepth checks if the depth is valid
func isValidDepth(depth types.SummaryTier) bool {
	switch depth {
	case types.TierTopic, types.TierDocument, types.TierChapter, types.TierParagraph, types.TierSource:
		return true
	}
	return false
}

// formatTopicList formats a list of topics for display
func formatTopicList(topics []types.TopicSummary) string {
	if len(topics) == 0 {
		return "No topics found."
	}

	text := "Available Topics:\n\n"
	for i, t := range topics {
		text += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, t.Name, t.ID)
		if t.Description != "" {
			text += fmt.Sprintf("   %s\n", t.Description)
		}
		if len(t.Tags) > 0 {
			text += fmt.Sprintf("   Tags: %v\n", t.Tags)
		}
		text += "\n"
	}
	return text
}

// formatTopic formats a single topic for display
func formatTopic(topic *types.TopicSummary) string {
	text := fmt.Sprintf(
		"Topic: %s\nID: %s\n",
		topic.Name,
		topic.ID,
	)

	if topic.Description != "" {
		text += fmt.Sprintf("Description: %s\n", topic.Description)
	}

	if len(topic.Tags) > 0 {
		text += fmt.Sprintf("Tags: %v\n", topic.Tags)
	}

	text += fmt.Sprintf("\nSummary:\n%s\n", topic.Summary)

	if len(topic.DocumentIDs) > 0 {
		text += fmt.Sprintf("\nAssociated Documents (%d):\n", len(topic.DocumentIDs))
		for i, docID := range topic.DocumentIDs {
			text += fmt.Sprintf("  %d. %s\n", i+1, docID)
		}
	}

	return text
}

// formatQueryResults formats query results for display
func formatQueryResults(question string, depth types.SummaryTier, results []types.QueryResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("Query: %s\nDepth: %s\n\nNo results found.", question, depth)
	}

	text := fmt.Sprintf("Query: %s\nDepth: %s\n\nResults:\n", question, depth)
	for i, r := range results {
		text += fmt.Sprintf("\n%d. %s\n   %s\n", i+1, r.DocumentTitle, r.Content)
	}
	return text
}

// handleHierarchicalQuery handles the tiersum_hierarchical_query tool
func (s *MCPServer) handleHierarchicalQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, ok := request.Params.Arguments["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	startTier := types.TierTopic
	if st, ok := request.Params.Arguments["start_tier"].(string); ok && st != "" {
		startTier = types.SummaryTier(st)
	}

	endTier := types.TierSource
	if et, ok := request.Params.Arguments["end_tier"].(string); ok && et != "" {
		endTier = types.SummaryTier(et)
	}

	maxResults := 10
	if mr, ok := request.Params.Arguments["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	req := types.HierarchicalQueryRequest{
		Question:   question,
		StartTier:  startTier,
		EndTier:    endTier,
		MaxResults: maxResults,
	}

	s.logger.Info("MCP hierarchical query", zap.String("question", question))

	// Try to cast to hierarchical query service
	svc, ok := s.queryService.(interface {
		HierarchicalQuery(ctx context.Context, req types.HierarchicalQueryRequest) (*types.HierarchicalQueryResponse, error)
	})
	if !ok {
		return nil, fmt.Errorf("query service does not support hierarchical query")
	}

	resp, err := svc.HierarchicalQuery(ctx, req)
	if err != nil {
		s.logger.Error("hierarchical query failed", zap.Error(err))
		return nil, fmt.Errorf("query failed: %w", err)
	}

	resultText := formatHierarchicalQueryResults(resp)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetTopicDocuments handles the tiersum_get_topic_documents tool
func (s *MCPServer) handleGetTopicDocuments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	topicID, ok := request.Params.Arguments["topic_id"].(string)
	if !ok || topicID == "" {
		return nil, fmt.Errorf("topic_id is required")
	}

	s.logger.Info("MCP get topic documents", zap.String("topic_id", topicID))

	svc, ok := s.topicService.(interface {
		GetTopicDocuments(ctx context.Context, topicID string) ([]types.Document, error)
	})
	if !ok {
		return nil, fmt.Errorf("topic service does not support GetTopicDocuments")
	}

	docs, err := svc.GetTopicDocuments(ctx, topicID)
	if err != nil {
		s.logger.Error("failed to get topic documents", zap.Error(err))
		return nil, fmt.Errorf("failed to get topic documents: %w", err)
	}

	resultText := formatTopicDocuments(topicID, docs)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetDocumentChapters handles the tiersum_get_document_chapters tool
func (s *MCPServer) handleGetDocumentChapters(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docID, ok := request.Params.Arguments["document_id"].(string)
	if !ok || docID == "" {
		return nil, fmt.Errorf("document_id is required")
	}

	s.logger.Info("MCP get document chapters", zap.String("document_id", docID))

	svc, ok := s.queryService.(interface {
		GetDocumentChapters(ctx context.Context, docID string) ([]types.Summary, error)
	})
	if !ok {
		return nil, fmt.Errorf("query service does not support GetDocumentChapters")
	}

	chapters, err := svc.GetDocumentChapters(ctx, docID)
	if err != nil {
		s.logger.Error("failed to get document chapters", zap.Error(err))
		return nil, fmt.Errorf("failed to get document chapters: %w", err)
	}

	resultText := formatDocumentChapters(docID, chapters)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetChapterParagraphs handles the tiersum_get_chapter_paragraphs tool
func (s *MCPServer) handleGetChapterParagraphs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chapterPath, ok := request.Params.Arguments["chapter_path"].(string)
	if !ok || chapterPath == "" {
		return nil, fmt.Errorf("chapter_path is required")
	}

	s.logger.Info("MCP get chapter paragraphs", zap.String("chapter_path", chapterPath))

	svc, ok := s.queryService.(interface {
		GetChapterParagraphs(ctx context.Context, chapterPath string) ([]types.Summary, error)
	})
	if !ok {
		return nil, fmt.Errorf("query service does not support GetChapterParagraphs")
	}

	paragraphs, err := svc.GetChapterParagraphs(ctx, chapterPath)
	if err != nil {
		s.logger.Error("failed to get chapter paragraphs", zap.Error(err))
		return nil, fmt.Errorf("failed to get chapter paragraphs: %w", err)
	}

	resultText := formatChapterParagraphs(chapterPath, paragraphs)
	return mcp.NewToolResultText(resultText), nil
}

// handleDrillDown handles the tiersum_drill_down tool
func (s *MCPServer) handleDrillDown(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	currentTier, ok := request.Params.Arguments["current_tier"].(string)
	if !ok || currentTier == "" {
		return nil, fmt.Errorf("current_tier is required")
	}

	parentID, ok := request.Params.Arguments["parent_id"].(string)
	if !ok || parentID == "" {
		return nil, fmt.Errorf("parent_id is required")
	}

	question, ok := request.Params.Arguments["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	s.logger.Info("MCP drill down",
		zap.String("tier", currentTier),
		zap.String("parent_id", parentID),
		zap.String("question", question))

	svc, ok := s.queryService.(interface {
		DrillDown(ctx context.Context, req types.DrillDownRequest) ([]types.QueryItem, error)
	})
	if !ok {
		return nil, fmt.Errorf("query service does not support DrillDown")
	}

	req := types.DrillDownRequest{
		DocumentID:  parentID,
		CurrentTier: types.SummaryTier(currentTier),
		Path:        parentID, // For chapters/paragraphs, path is same as ID
		Question:    question,
	}

	items, err := svc.DrillDown(ctx, req)
	if err != nil {
		s.logger.Error("drill down failed", zap.Error(err))
		return nil, fmt.Errorf("drill down failed: %w", err)
	}

	resultText := formatDrillDownResults(currentTier, items)
	return mcp.NewToolResultText(resultText), nil
}

// handleGetSource handles the tiersum_get_source tool
func (s *MCPServer) handleGetSource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docID, ok := request.Params.Arguments["document_id"].(string)
	if !ok || docID == "" {
		return nil, fmt.Errorf("document_id is required")
	}

	path, ok := request.Params.Arguments["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	s.logger.Info("MCP get source", zap.String("document_id", docID), zap.String("path", path))

	svc, ok := s.queryService.(interface {
		GetSource(ctx context.Context, docID string, path string) (*types.QueryItem, error)
	})
	if !ok {
		return nil, fmt.Errorf("query service does not support GetSource")
	}

	item, err := svc.GetSource(ctx, docID, path)
	if err != nil {
		s.logger.Error("failed to get source", zap.Error(err))
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	if item == nil {
		return mcp.NewToolResultText("Source not found"), nil
	}

	resultText := formatSource(item)
	return mcp.NewToolResultText(resultText), nil
}

// formatHierarchicalQueryResults formats hierarchical query results
func formatHierarchicalQueryResults(resp *types.HierarchicalQueryResponse) string {
	if len(resp.Levels) == 0 {
		return fmt.Sprintf("Query: %s\n\nNo results found.", resp.Question)
	}

	text := fmt.Sprintf("Query: %s\n\nHierarchical Results:\n", resp.Question)

	for _, level := range resp.Levels {
		text += fmt.Sprintf("\n=== %s Level ===\n", level.Tier)
		if len(level.Items) == 0 {
			text += "  (No items at this level)\n"
			continue
		}
		for i, item := range level.Items {
			text += fmt.Sprintf("\n%d. %s (relevance: %.2f)\n", i+1, item.Title, item.Relevance)
			text += fmt.Sprintf("   Path: %s\n", item.Path)
			if item.IsSource {
				text += "   [SOURCE - cannot drill down further]\n"
			} else {
				text += fmt.Sprintf("   [Can drill down: %d child items]\n", item.ChildCount)
			}
			content := truncateString(item.Content, 300)
			text += fmt.Sprintf("   %s\n", content)
		}
		if level.HasMore {
			text += "\n   ... (more items available)\n"
		}
	}

	return text
}

// formatTopicDocuments formats topic documents for display
func formatTopicDocuments(topicID string, docs []types.Document) string {
	if len(docs) == 0 {
		return fmt.Sprintf("Topic ID: %s\n\nNo documents found.", topicID)
	}

	text := fmt.Sprintf("Topic ID: %s\n\nDocuments (%d):\n\n", topicID, len(docs))
	for i, d := range docs {
		text += fmt.Sprintf("%d. %s (ID: %s)\n", i+1, d.Title, d.ID)
		if len(d.Tags) > 0 {
			text += fmt.Sprintf("   Tags: %v\n", d.Tags)
		}
		text += fmt.Sprintf("   Preview: %s\n\n", truncateString(d.Content, 200))
	}
	return text
}

// formatDocumentChapters formats document chapters for display
func formatDocumentChapters(docID string, chapters []types.Summary) string {
	if len(chapters) == 0 {
		return fmt.Sprintf("Document ID: %s\n\nNo chapters found.", docID)
	}

	text := fmt.Sprintf("Document ID: %s\n\nChapters/Sections (%d):\n\n", docID, len(chapters))
	for i, ch := range chapters {
		title := extractTitleFromPath(ch.Path)
		text += fmt.Sprintf("%d. %s\n", i+1, title)
		text += fmt.Sprintf("   Path: %s\n", ch.Path)
		if ch.IsSource {
			text += "   [This is a short section - source level]\n"
		}
		text += fmt.Sprintf("   %s\n\n", truncateString(ch.Content, 250))
	}
	return text
}

// formatChapterParagraphs formats chapter paragraphs for display
func formatChapterParagraphs(chapterPath string, paragraphs []types.Summary) string {
	if len(paragraphs) == 0 {
		return fmt.Sprintf("Chapter Path: %s\n\nNo paragraphs found.", chapterPath)
	}

	text := fmt.Sprintf("Chapter Path: %s\n\nParagraphs (%d):\n\n", chapterPath, len(paragraphs))
	for i, p := range paragraphs {
		text += fmt.Sprintf("%d. Paragraph %s\n", i+1, extractLastSegment(p.Path))
		text += fmt.Sprintf("   Path: %s\n", p.Path)
		text += fmt.Sprintf("   %s\n\n", truncateString(p.Content, 300))
	}
	return text
}

// formatDrillDownResults formats drill-down results
func formatDrillDownResults(fromTier string, items []types.QueryItem) string {
	if len(items) == 0 {
		return fmt.Sprintf("Drill-down from %s: No relevant items found.", fromTier)
	}

	nextTier := getNextTier(types.SummaryTier(fromTier))
	text := fmt.Sprintf("Drill-down Results: %s → %s (%d items)\n\n", fromTier, nextTier, len(items))

	for i, item := range items {
		text += fmt.Sprintf("%d. %s (relevance: %.2f)\n", i+1, item.Title, item.Relevance)
		text += fmt.Sprintf("   ID: %s\n", item.ID)
		text += fmt.Sprintf("   Path: %s\n", item.Path)
		if item.IsSource {
			text += "   [SOURCE - final level]\n"
		} else {
			text += fmt.Sprintf("   [Can drill down further: %d children]\n", item.ChildCount)
		}
		content := truncateString(item.Content, 250)
		text += fmt.Sprintf("   %s\n\n", content)
	}

	return text
}

// formatSource formats source content for display
func formatSource(item *types.QueryItem) string {
	return fmt.Sprintf(
		"Source Content\n==============\nPath: %s\nTier: %s\n\n%s",
		item.Path,
		item.Tier,
		item.Content,
	)
}

// getNextTier returns the next tier in hierarchy
func getNextTier(current types.SummaryTier) types.SummaryTier {
	switch current {
	case types.TierTopic:
		return types.TierDocument
	case types.TierDocument:
		return types.TierChapter
	case types.TierChapter:
		return types.TierParagraph
	case types.TierParagraph:
		return types.TierSource
	}
	return ""
}

// extractTitleFromPath extracts a readable title from path
func extractTitleFromPath(path string) string {
	parts := splitPath(path)
	if len(parts) == 0 {
		return path
	}
	if len(parts) == 1 {
		return parts[0]
	}
	lastPart := parts[len(parts)-1]
	if isNumber(lastPart) && len(parts) > 1 {
		return fmt.Sprintf("%s (Section %s)", parts[len(parts)-2], lastPart)
	}
	return lastPart
}

// extractLastSegment extracts the last segment of a path
func extractLastSegment(path string) string {
	parts := splitPath(path)
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

// splitPath splits path by "/"
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	for _, p := range splitByChar(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// splitByChar splits string by character
func splitByChar(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// isNumber checks if string is a number
func isNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// formatDocument formats document info for display
func formatDocument(doc *types.Document) string {
	return fmt.Sprintf(
		"Document: %s\nID: %s\nFormat: %s\nCreated: %s\n\nContent Preview:\n%s",
		doc.Title,
		doc.ID,
		doc.Format,
		doc.CreatedAt.Format("2006-01-02 15:04:05"),
		truncateString(doc.Content, 500),
	)
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
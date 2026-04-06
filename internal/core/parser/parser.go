package parser

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Node represents a node in the document hierarchy
type Node struct {
	Level    int     `json:"level"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
	Children []*Node `json:"children,omitempty"`
	Path     string  `json:"path"`
}

// Document represents a parsed document
type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Root    *Node  `json:"root"`
}

// Parser handles markdown parsing
type Parser struct {
	md goldmark.Markdown
}

// New creates a new parser
func New() *Parser {
	return &Parser{
		md: goldmark.New(),
	}
}

// Parse parses markdown content into a hierarchical document
func (p *Parser) Parse(content string) (*Document, error) {
	source := []byte(content)
	reader := text.NewReader(source)
	doc := p.md.Parser().Parse(reader)

	root := &Node{
		Level:   0,
		Title:   "Document Root",
		Content: "",
		Path:    "",
	}

	document := &Document{
		Root: root,
	}

	// Walk the AST to extract headings and content
	var currentNode = root
	var nodeStack []*Node

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			level := node.Level
			title := string(node.Text(source))

			newNode := &Node{
				Level:   level,
				Title:   title,
				Content: "",
			}

			// Find parent based on level
			for len(nodeStack) > 0 && nodeStack[len(nodeStack)-1].Level >= level {
				nodeStack = nodeStack[:len(nodeStack)-1]
			}

			if len(nodeStack) > 0 {
				parent := nodeStack[len(nodeStack)-1]
				newNode.Path = parent.Path + "." + string(rune('0'+len(parent.Children)))
				parent.Children = append(parent.Children, newNode)
			} else {
				newNode.Path = string(rune('0' + len(root.Children)))
				root.Children = append(root.Children, newNode)
			}

			nodeStack = append(nodeStack, newNode)
			currentNode = newNode

			// Set document title from first H1
			if level == 1 && document.Title == "" {
				document.Title = title
			}

		case *ast.Paragraph:
			if currentNode != nil {
				text := string(node.Text(source))
				currentNode.Content += text + "\n"
			}
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, err
	}

	return document, nil
}

// ExtractParagraphs extracts all paragraphs from content
func (p *Parser) ExtractParagraphs(content string) []string {
	var paragraphs []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	var current strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if current.Len() > 0 {
				paragraphs = append(paragraphs, strings.TrimSpace(current.String()))
				current.Reset()
			}
		} else {
			if current.Len() > 0 {
				current.WriteString(" ")
			}
			current.WriteString(line)
		}
	}

	if current.Len() > 0 {
		paragraphs = append(paragraphs, strings.TrimSpace(current.String()))
	}

	return paragraphs
}

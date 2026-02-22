package mcp

// ToolDefinition defines an MCP tool.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func getToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "search",
			Description: "Search indexed documents using semantic or full-text search",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query text"},
					"mode":  map[string]interface{}{"type": "string", "enum": []string{"semantic", "fulltext"}, "description": "Search mode (default: semantic)"},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Filter by tag names (AND logic)",
					},
					"limit": map[string]interface{}{"type": "integer", "description": "Maximum number of results (default: 10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "index_file",
			Description: "Index a file by its path into the search index",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Absolute path to the file to index"},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Tag names to assign (auto-tagging also runs)",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "get_document",
			Description: "Get full details of an indexed document by ID or file path",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":   map[string]interface{}{"type": "string", "description": "Document UUID"},
					"path": map[string]interface{}{"type": "string", "description": "Document file path"},
				},
			},
		},
		{
			Name:        "list_tags",
			Description: "List all document tags",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "delete_document",
			Description: "Delete a document and all its indexed data",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":   map[string]interface{}{"type": "string", "description": "Document UUID"},
					"path": map[string]interface{}{"type": "string", "description": "Document file path"},
				},
			},
		},
		{
			Name:        "status",
			Description: "Get index statistics (document counts, chunk counts, breakdowns)",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "update",
			Description: "Re-index modified documents. Checks file mtime and re-indexes changed files.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":  map[string]interface{}{"type": "string", "description": "Specific file path to update (omit for all)"},
					"force": map[string]interface{}{"type": "boolean", "description": "Force re-index regardless of mtime (default: false)"},
				},
			},
		},
	}
}

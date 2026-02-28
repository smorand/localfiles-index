package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"localfiles-index/internal/analyzer"
	"localfiles-index/internal/config"
	"localfiles-index/internal/embedding"
	"localfiles-index/internal/gdrive"
	"localfiles-index/internal/indexer"
	"localfiles-index/internal/searcher"
	"localfiles-index/internal/storage"
)

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// Server implements an MCP HTTP Streamable server.
type Server struct {
	app          *fiber.App
	store        *storage.Store
	cfg          *config.Config
	creds        *OAuthCredentials
	tokenStore   *TokenStore
	port         int
	gdriveClient *gdrive.Client // lazily initialized
}

// NewServer creates a new MCP HTTP server.
func NewServer(store *storage.Store, cfg *config.Config, creds *OAuthCredentials, port int) *Server {
	s := &Server{
		store:      store,
		cfg:        cfg,
		creds:      creds,
		tokenStore: NewTokenStore(1 * time.Hour),
		port:       port,
	}

	s.app = fiber.New(fiber.Config{
		AppName:      "localfiles-index-mcp",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	s.setupRoutes()
	return s
}

// Start starts the MCP HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("starting MCP server", "port", s.port)
	return s.app.Listen(addr)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

// getGDriveClient lazily initializes the Google Drive client.
// In server mode, requires a pre-cached token (no browser flow).
func (s *Server) getGDriveClient(ctx context.Context) (*gdrive.Client, error) {
	if s.gdriveClient != nil {
		return s.gdriveClient, nil
	}

	if s.cfg.GoogleCredentialsFile == "" {
		return nil, fmt.Errorf("GOOGLE_CREDENTIALS_FILE not configured (required for Google Drive operations)")
	}

	oauthConfig, err := gdrive.LoadOAuthConfig(s.cfg.GoogleCredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("loading Google credentials: %w", err)
	}

	// In server mode, use cached token only (no browser flow)
	token, err := gdrive.GetCachedToken(ctx, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("Google Drive auth: %w", err)
	}

	client, err := gdrive.NewClient(ctx, oauthConfig, token)
	if err != nil {
		return nil, fmt.Errorf("creating Google Drive client: %w", err)
	}

	s.gdriveClient = client
	return client, nil
}

func (s *Server) setupRoutes() {
	// OAuth endpoints
	s.app.Post("/oauth/token", s.handleToken)
	s.app.Get("/oauth/callback", s.handleOAuthCallback)

	// MCP endpoint (HTTP Streamable)
	s.app.Post("/mcp", s.authMiddleware, s.handleMCP)

	// REST API endpoints
	api := s.app.Group("/api", s.authMiddleware)
	api.Get("/documents", s.apiSearchDocuments)
	api.Post("/documents", s.apiIndexDocument)
	api.Get("/documents/:id", s.apiGetDocument)
	api.Put("/documents/:id", s.apiUpdateDocument)
	api.Put("/documents", s.apiUpdateAllDocuments)
	api.Delete("/documents/:id", s.apiDeleteDocument)
	api.Get("/tags", s.apiListTags)
	api.Get("/tags/:name", s.apiGetTag)
	api.Post("/tags", s.apiCreateTag)
	api.Put("/tags/:name", s.apiUpdateTag)
	api.Delete("/tags/:name", s.apiDeleteTag)
	api.Get("/status", s.apiStatus)

	// Health check
	s.app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}

// authMiddleware validates the Bearer token.
func (s *Server) authMiddleware(c fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" {
		return c.Status(401).JSON(fiber.Map{
			"error":             "unauthorized",
			"error_description": "missing authorization header",
		})
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return c.Status(401).JSON(fiber.Map{
			"error":             "unauthorized",
			"error_description": "invalid authorization scheme",
		})
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if !s.tokenStore.ValidateToken(token) {
		return c.Status(401).JSON(fiber.Map{
			"error":             "unauthorized",
			"error_description": "invalid or expired token",
		})
	}

	return c.Next()
}

// handleToken implements OAuth 2.1 client credentials grant.
func (s *Server) handleToken(c fiber.Ctx) error {
	grantType := c.FormValue("grant_type")
	clientID := c.FormValue("client_id")
	clientSecret := c.FormValue("client_secret")

	if grantType != "client_credentials" {
		return c.Status(400).JSON(fiber.Map{
			"error":             "unsupported_grant_type",
			"error_description": "only client_credentials grant type is supported",
		})
	}

	if clientID != s.creds.ClientID || clientSecret != s.creds.ClientSecret {
		return c.Status(401).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "invalid client credentials",
		})
	}

	token, expiry := s.tokenStore.IssueToken()
	return c.JSON(fiber.Map{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(expiry.Seconds()),
	})
}

// handleOAuthCallback handles the OAuth 2.1 callback redirect.
func (s *Server) handleOAuthCallback(c fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")

	if errorParam != "" {
		errorDesc := c.Query("error_description")
		return c.Status(400).JSON(fiber.Map{
			"error":             errorParam,
			"error_description": errorDesc,
		})
	}

	if code == "" {
		return c.Status(400).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "missing authorization code",
		})
	}

	// Exchange authorization code for token
	token, expiry := s.tokenStore.IssueToken()
	return c.JSON(fiber.Map{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(expiry.Seconds()),
		"state":        state,
	})
}

// MCPRequest represents an MCP JSON-RPC request.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// MCPResponse represents an MCP JSON-RPC response.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolCallParams holds the parameters for a tools/call request.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult holds the result of a tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a piece of content in a tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// handleMCP handles MCP JSON-RPC requests.
func (s *Server) handleMCP(c fiber.Ctx) error {
	var req MCPRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.JSON(MCPResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &MCPError{Code: -32700, Message: "parse error"},
		})
	}

	var result interface{}
	var mcpErr *MCPError

	switch req.Method {
	case "initialize":
		result = s.handleInitialize()
	case "tools/list":
		result = s.handleToolsList()
	case "tools/call":
		result, mcpErr = s.handleToolsCall(c.Context(), req.Params)
	default:
		mcpErr = &MCPError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}
	if mcpErr != nil {
		resp.Error = mcpErr
	} else {
		resp.Result = result
	}

	return c.JSON(resp)
}

func (s *Server) handleInitialize() interface{} {
	return fiber.Map{
		"protocolVersion": "2025-03-26",
		"capabilities": fiber.Map{
			"tools": fiber.Map{},
		},
		"serverInfo": fiber.Map{
			"name":    "localfiles-index",
			"version": "1.0.0",
		},
	}
}

func (s *Server) handleToolsList() interface{} {
	return fiber.Map{
		"tools": getToolDefinitions(),
	}
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, *MCPError) {
	var callParams ToolCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &MCPError{Code: -32602, Message: "invalid params"}
	}

	var args map[string]interface{}
	if err := json.Unmarshal(callParams.Arguments, &args); err != nil {
		return nil, &MCPError{Code: -32602, Message: "invalid tool arguments"}
	}

	slog.Debug("MCP tool call", "tool", callParams.Name, "args", args)

	switch callParams.Name {
	case "search":
		return s.toolSearch(ctx, args)
	case "index_file":
		return s.toolIndexFile(ctx, args)
	case "get_document":
		return s.toolGetDocument(ctx, args)
	case "list_tags":
		return s.toolListTags(ctx)
	case "delete_document":
		return s.toolDeleteDocument(ctx, args)
	case "status":
		return s.toolStatus(ctx)
	case "update":
		return s.toolUpdate(ctx, args)
	default:
		return nil, &MCPError{Code: -32602, Message: fmt.Sprintf("unknown tool: %s", callParams.Name)}
	}
}

func textResult(data interface{}) *ToolResult {
	jsonBytes, _ := json.Marshal(data)
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(jsonBytes)}},
	}
}

func errorResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func (s *Server) toolSearch(ctx context.Context, args map[string]interface{}) (*ToolResult, *MCPError) {
	query, _ := args["query"].(string)
	if query == "" {
		return errorResult("query parameter is required"), nil
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "semantic"
	}

	// Parse tags from args (can be array or comma-separated string)
	var tags []string
	if tagsRaw, ok := args["tags"]; ok {
		switch v := tagsRaw.(type) {
		case []interface{}:
			for _, t := range v {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
		case string:
			if v != "" {
				for _, t := range strings.Split(v, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
		}
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Validate tags
	for _, t := range tags {
		if _, err := s.store.GetTagByName(ctx, t); err != nil {
			return errorResult(fmt.Sprintf("tag not found: %s", t)), nil
		}
	}

	emb, err := embedding.New(ctx, s.cfg.GeminiAPIKey, s.cfg.EmbeddingModel, s.cfg.EmbeddingDimensions)
	if err != nil {
		return errorResult(fmt.Sprintf("creating embedder: %v", err)), nil
	}

	srch := searcher.New(s.store, emb)
	results, err := srch.Search(ctx, query, mode, tags, limit)
	if err != nil {
		return errorResult(fmt.Sprintf("search failed: %v", err)), nil
	}

	return textResult(results), nil
}

func (s *Server) toolIndexFile(ctx context.Context, args map[string]interface{}) (*ToolResult, *MCPError) {
	path, _ := args["path"].(string)
	if path == "" {
		return errorResult("path parameter is required"), nil
	}

	// Parse tags
	var tags []string
	if tagsRaw, ok := args["tags"]; ok {
		switch v := tagsRaw.(type) {
		case []interface{}:
			for _, t := range v {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
		case string:
			if v != "" {
				for _, t := range strings.Split(v, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
		}
	}

	anlz := analyzer.New(s.cfg.OpenRouterAPIKey, s.cfg.InferenceModel)

	emb, err := embedding.New(ctx, s.cfg.GeminiAPIKey, s.cfg.EmbeddingModel, s.cfg.EmbeddingDimensions)
	if err != nil {
		return errorResult(fmt.Sprintf("creating embedder: %v", err)), nil
	}

	idx := indexer.New(s.store, anlz, emb, s.cfg)

	var result *indexer.IndexResult

	if gdrive.IsGDrivePath(path) {
		gdriveClient, err := s.getGDriveClient(ctx)
		if err != nil {
			return errorResult(fmt.Sprintf("Google Drive client: %v", err)), nil
		}
		fileID := gdrive.ExtractFileID(path)
		result, err = idx.IndexGDriveFile(ctx, gdriveClient, fileID, tags)
		if err != nil {
			return errorResult(fmt.Sprintf("indexing GDrive file failed: %v", err)), nil
		}
	} else {
		result, err = idx.IndexFile(ctx, path, tags)
		if err != nil {
			return errorResult(fmt.Sprintf("indexing failed: %v", err)), nil
		}
	}

	return textResult(fiber.Map{
		"document_id": result.DocumentID,
		"title":       result.Title,
		"chunks":      result.ChunkCount,
		"images":      result.ImageCount,
		"tags":        result.Tags,
	}), nil
}

func (s *Server) toolGetDocument(ctx context.Context, args map[string]interface{}) (*ToolResult, *MCPError) {
	idStr, _ := args["id"].(string)
	path, _ := args["path"].(string)

	if idStr == "" && path == "" {
		return errorResult("id or path parameter is required"), nil
	}

	var doc *storage.Document
	var err error

	if path != "" {
		doc, err = s.store.GetDocumentByPath(ctx, path)
	} else {
		id, parseErr := parseUUID(idStr)
		if parseErr != nil {
			return errorResult(fmt.Sprintf("invalid id: %s", idStr)), nil
		}
		doc, err = s.store.GetDocumentWithChunks(ctx, id)
	}

	if err != nil {
		return errorResult(fmt.Sprintf("document not found: %v", err)), nil
	}

	// Get full document with chunks
	fullDoc, err := s.store.GetDocumentWithChunks(ctx, doc.ID)
	if err != nil {
		return errorResult(fmt.Sprintf("loading document details: %v", err)), nil
	}

	result := fiber.Map{
		"id":               fullDoc.ID,
		"file_path":        fullDoc.FilePath,
		"title":            fullDoc.Title,
		"title_confidence": fullDoc.TitleConfidence,
		"document_type":    fullDoc.DocumentType,
		"mime_type":        fullDoc.MimeType,
		"file_size":        fullDoc.FileSize,
		"indexed_at":       fullDoc.IndexedAt,
		"chunks":           len(fullDoc.Chunks),
		"images":           len(fullDoc.Images),
	}

	if len(fullDoc.Tags) > 0 {
		var tagNames []string
		for _, t := range fullDoc.Tags {
			tagNames = append(tagNames, t.Name)
		}
		result["tags"] = tagNames
	}

	return textResult(result), nil
}

func (s *Server) toolListTags(ctx context.Context) (*ToolResult, *MCPError) {
	tags, err := s.store.ListTags(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("listing tags: %v", err)), nil
	}

	return textResult(tags), nil
}

func (s *Server) toolDeleteDocument(ctx context.Context, args map[string]interface{}) (*ToolResult, *MCPError) {
	idStr, _ := args["id"].(string)
	path, _ := args["path"].(string)

	if idStr == "" && path == "" {
		return errorResult("id or path parameter is required"), nil
	}

	var doc *storage.Document
	var err error

	if path != "" {
		doc, err = s.store.GetDocumentByPath(ctx, path)
	} else {
		id, parseErr := parseUUID(idStr)
		if parseErr != nil {
			return errorResult(fmt.Sprintf("invalid id: %s", idStr)), nil
		}
		doc, err = s.store.GetDocumentByID(ctx, id)
	}

	if err != nil {
		return errorResult(fmt.Sprintf("document not found: %v", err)), nil
	}

	if err := s.store.DeleteDocument(ctx, doc.ID); err != nil {
		return errorResult(fmt.Sprintf("deleting document: %v", err)), nil
	}

	return textResult(fiber.Map{
		"deleted":     true,
		"document_id": doc.ID,
		"file_path":   doc.FilePath,
	}), nil
}

func (s *Server) toolStatus(ctx context.Context) (*ToolResult, *MCPError) {
	stats, err := s.store.GetStats(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("getting stats: %v", err)), nil
	}

	return textResult(stats), nil
}

func (s *Server) toolUpdate(ctx context.Context, args map[string]interface{}) (*ToolResult, *MCPError) {
	path, _ := args["path"].(string)
	force, _ := args["force"].(bool)

	anlz := analyzer.New(s.cfg.OpenRouterAPIKey, s.cfg.InferenceModel)

	emb, err := embedding.New(ctx, s.cfg.GeminiAPIKey, s.cfg.EmbeddingModel, s.cfg.EmbeddingDimensions)
	if err != nil {
		return errorResult(fmt.Sprintf("creating embedder: %v", err)), nil
	}

	idx := indexer.New(s.store, anlz, emb, s.cfg)

	if path != "" {
		// Update single file
		doc, err := s.store.GetDocumentByPath(ctx, path)
		if err != nil {
			return errorResult(fmt.Sprintf("document not found in index: %s", path)), nil
		}

		result, _, err := s.updateSingleDocument(ctx, idx, doc, force)
		if err != nil {
			return errorResult(fmt.Sprintf("update failed: %v", err)), nil
		}

		return textResult(result), nil
	}

	// Update all
	return s.updateAllDocuments(ctx, idx, force)
}

func (s *Server) updateSingleDocument(ctx context.Context, idx *indexer.Indexer, doc *storage.Document, force bool) (fiber.Map, bool, error) {
	// Preserve existing tags
	var tagNames []string
	for _, t := range doc.Tags {
		tagNames = append(tagNames, t.Name)
	}

	if gdrive.IsGDrivePath(doc.FilePath) {
		gdriveClient, err := s.getGDriveClient(ctx)
		if err != nil {
			return nil, false, err
		}

		fileID := gdrive.ExtractFileID(doc.FilePath)
		info, err := gdriveClient.GetFileInfo(ctx, fileID)
		if err != nil {
			return fiber.Map{"updated": 0, "unchanged": 0, "missing": 1}, false, nil
		}

		if !force && !info.ModifiedTime.After(doc.FileMtime) {
			return fiber.Map{"updated": 0, "unchanged": 1, "missing": 0}, false, nil
		}

		if _, err := idx.IndexGDriveFile(ctx, gdriveClient, fileID, tagNames); err != nil {
			return nil, false, err
		}

		return fiber.Map{"updated": 1, "unchanged": 0, "missing": 0}, true, nil
	}

	stat, err := os.Stat(doc.FilePath)
	if err != nil {
		return fiber.Map{"updated": 0, "unchanged": 0, "missing": 1}, false, nil
	}

	if !force && !stat.ModTime().After(doc.FileMtime) {
		return fiber.Map{"updated": 0, "unchanged": 1, "missing": 0}, false, nil
	}

	if _, err := idx.IndexFile(ctx, doc.FilePath, tagNames); err != nil {
		return nil, false, err
	}

	return fiber.Map{"updated": 1, "unchanged": 0, "missing": 0}, true, nil
}

func (s *Server) updateAllDocuments(ctx context.Context, idx *indexer.Indexer, force bool) (*ToolResult, *MCPError) {
	docs, err := s.store.ListDocuments(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("listing documents: %v", err)), nil
	}

	var updated, unchanged, missing int

	for _, doc := range docs {
		// Preserve existing tags
		var tagNames []string
		for _, t := range doc.Tags {
			tagNames = append(tagNames, t.Name)
		}

		if gdrive.IsGDrivePath(doc.FilePath) {
			gdriveClient, gErr := s.getGDriveClient(ctx)
			if gErr != nil {
				slog.Warn("skipping GDrive document (no credentials)", "path", doc.FilePath)
				missing++
				continue
			}

			fileID := gdrive.ExtractFileID(doc.FilePath)
			info, gErr := gdriveClient.GetFileInfo(ctx, fileID)
			if gErr != nil {
				slog.Warn("failed to get GDrive file info", "path", doc.FilePath, "error", gErr)
				missing++
				continue
			}

			if !force && !info.ModifiedTime.After(doc.FileMtime) {
				unchanged++
				continue
			}

			if _, gErr := idx.IndexGDriveFile(ctx, gdriveClient, fileID, tagNames); gErr != nil {
				slog.Error("failed to re-index GDrive file", "path", doc.FilePath, "error", gErr)
				continue
			}

			updated++
			continue
		}

		stat, err := os.Stat(doc.FilePath)
		if err != nil {
			slog.Warn("file missing from disk", "path", doc.FilePath)
			missing++
			continue
		}

		if !force && !stat.ModTime().After(doc.FileMtime) {
			unchanged++
			continue
		}

		if _, err := idx.IndexFile(ctx, doc.FilePath, tagNames); err != nil {
			slog.Error("failed to re-index", "path", doc.FilePath, "error", err)
			continue
		}

		updated++
	}

	return textResult(fiber.Map{
		"updated":   updated,
		"unchanged": unchanged,
		"missing":   missing,
	}), nil
}

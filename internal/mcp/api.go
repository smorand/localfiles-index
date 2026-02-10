package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// sendToolResult converts a ToolResult (from MCP tool handlers) into an HTTP JSON response.
// If mcpErr is non-nil, returns 500. If result.IsError, returns 400. Otherwise returns 200.
func (s *Server) sendToolResult(c fiber.Ctx, result *ToolResult, mcpErr *MCPError) error {
	if mcpErr != nil {
		return c.Status(500).JSON(fiber.Map{"error": mcpErr.Message})
	}
	if result == nil {
		return c.Status(500).JSON(fiber.Map{"error": "no result"})
	}
	if result.IsError {
		msg := ""
		if len(result.Content) > 0 {
			msg = result.Content[0].Text
		}
		return c.Status(400).JSON(fiber.Map{"error": msg})
	}
	// Unwrap the JSON text embedded in Content[0].Text
	if len(result.Content) > 0 {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(result.Content[0].Text), &raw); err == nil {
			return c.Status(200).Send(raw)
		}
		// Fallback: return as string
		return c.JSON(fiber.Map{"result": result.Content[0].Text})
	}
	return c.JSON(fiber.Map{})
}

// --- Document Endpoints ---

// apiSearchDocuments handles GET /api/documents?query=...&mode=...&category=...&limit=...
func (s *Server) apiSearchDocuments(c fiber.Ctx) error {
	query := c.Query("query")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "query parameter is required"})
	}
	args := map[string]interface{}{
		"query": query,
	}
	if mode := c.Query("mode"); mode != "" {
		args["mode"] = mode
	}
	if cat := c.Query("category"); cat != "" {
		args["category"] = cat
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			args["limit"] = float64(limit)
		}
	}
	result, mcpErr := s.toolSearch(c.Context(), args)
	return s.sendToolResult(c, result, mcpErr)
}

// apiIndexDocument handles POST /api/documents
func (s *Server) apiIndexDocument(c fiber.Ctx) error {
	var body struct {
		Path     string `json:"path"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	args := map[string]interface{}{
		"path":     body.Path,
		"category": body.Category,
	}
	result, mcpErr := s.toolIndexFile(c.Context(), args)
	return s.sendToolResult(c, result, mcpErr)
}

// apiGetDocument handles GET /api/documents/:id
func (s *Server) apiGetDocument(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "document id is required"})
	}
	args := map[string]interface{}{
		"id": id,
	}
	result, mcpErr := s.toolGetDocument(c.Context(), args)
	return s.sendToolResult(c, result, mcpErr)
}

// apiUpdateDocument handles PUT /api/documents/:id — re-index a single document by UUID
func (s *Server) apiUpdateDocument(c fiber.Ctx) error {
	idStr := c.Params("id")
	if idStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "document id is required"})
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("invalid id: %s", idStr)})
	}
	// Look up document to get its file path
	doc, err := s.store.GetDocumentByID(c.Context(), id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("document not found: %v", err)})
	}

	var body struct {
		Force bool `json:"force"`
	}
	if len(c.Body()) > 0 {
		json.Unmarshal(c.Body(), &body)
	}

	args := map[string]interface{}{
		"path":  doc.FilePath,
		"force": body.Force,
	}
	result, mcpErr := s.toolUpdate(c.Context(), args)
	return s.sendToolResult(c, result, mcpErr)
}

// apiUpdateAllDocuments handles PUT /api/documents — re-scan all documents
func (s *Server) apiUpdateAllDocuments(c fiber.Ctx) error {
	var body struct {
		Force bool `json:"force"`
	}
	if len(c.Body()) > 0 {
		json.Unmarshal(c.Body(), &body)
	}
	args := map[string]interface{}{
		"force": body.Force,
	}
	result, mcpErr := s.toolUpdate(c.Context(), args)
	return s.sendToolResult(c, result, mcpErr)
}

// apiDeleteDocument handles DELETE /api/documents/:id
func (s *Server) apiDeleteDocument(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "document id is required"})
	}
	args := map[string]interface{}{
		"id": id,
	}
	result, mcpErr := s.toolDeleteDocument(c.Context(), args)
	return s.sendToolResult(c, result, mcpErr)
}

// --- Category Endpoints ---

// apiListCategories handles GET /api/categories
func (s *Server) apiListCategories(c fiber.Ctx) error {
	cats, err := s.store.ListCategories(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("listing categories: %v", err)})
	}
	return c.JSON(cats)
}

// apiGetCategory handles GET /api/categories/:name
func (s *Server) apiGetCategory(c fiber.Ctx) error {
	name := c.Params("name")
	cat, err := s.store.GetCategoryByName(c.Context(), name)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("category not found: %s", name)})
	}
	return c.JSON(cat)
}

// apiCreateCategory handles POST /api/categories
func (s *Server) apiCreateCategory(c fiber.Ctx) error {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if body.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	cat, err := s.store.CreateCategory(c.Context(), body.Name, body.Description)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("creating category: %v", err)})
	}
	return c.Status(201).JSON(cat)
}

// apiUpdateCategory handles PUT /api/categories/:name
func (s *Server) apiUpdateCategory(c fiber.Ctx) error {
	name := c.Params("name")
	var body struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	cat, err := s.store.UpdateCategory(c.Context(), name, body.Description)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("updating category: %v", err)})
	}
	return c.JSON(cat)
}

// apiDeleteCategory handles DELETE /api/categories/:name
func (s *Server) apiDeleteCategory(c fiber.Ctx) error {
	name := c.Params("name")
	newCategory := c.Query("new_category")
	if err := s.store.DeleteCategory(c.Context(), name, newCategory); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("deleting category: %v", err)})
	}
	return c.JSON(fiber.Map{"deleted": true, "name": name})
}

// --- Status Endpoint ---

// apiStatus handles GET /api/status
func (s *Server) apiStatus(c fiber.Ctx) error {
	result, mcpErr := s.toolStatus(c.Context())
	return s.sendToolResult(c, result, mcpErr)
}

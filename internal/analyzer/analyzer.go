package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/genai"
)

// Analyzer performs AI-based content analysis using Gemini.
type Analyzer struct {
	client *genai.Client
	model  string
}

// New creates a new Analyzer.
func New(ctx context.Context, apiKey string, model string) (*Analyzer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	return &Analyzer{
		client: client,
		model:  model,
	}, nil
}

// ImageAnalysisResult holds the result of analyzing an image.
type ImageAnalysisResult struct {
	ContentType string    `json:"content_type"`
	Title       string    `json:"title"`
	Confidence  float64   `json:"confidence"`
	Description string    `json:"description"`
	Segments    []Segment `json:"segments"`
}

// Segment holds a single searchable segment from AI analysis.
type Segment struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

// TitleResult holds a generated title with confidence.
type TitleResult struct {
	Title      string  `json:"title"`
	Confidence float64 `json:"confidence"`
}

// SpreadsheetResult holds the result of analyzing a spreadsheet.
type SpreadsheetResult struct {
	Title       string  `json:"title"`
	Confidence  float64 `json:"confidence"`
	Summary     string  `json:"summary"`
	Description string  `json:"description"`
}

// AnalyzeImage analyzes an image file and returns structured segments.
func (a *Analyzer) AnalyzeImage(ctx context.Context, imagePath string) (*ImageAnalysisResult, error) {
	slog.Debug("analyzing image", "path", imagePath)

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("reading image file: %w", err)
	}

	prompt := `Analyze this image and return a JSON response with the following structure:
{
  "content_type": "<type of content: official_document, photograph, diagram, chart, screenshot, illustration, etc.>",
  "title": "<descriptive title for this image>",
  "confidence": <confidence score 0.0 to 1.0 for the title>,
  "description": "<full description of the image>",
  "segments": [
    {"label": "<segment_label_snake_case>", "content": "<searchable text content for this segment>"}
  ]
}

Rules for segments:
- For official documents (passport, ID card, invoice, etc.): create segments for each key identifier (document_number, holder_name, dates, amounts, etc.) plus a full description
- For photographs: create 1-2 segments (scene_description, and optionally people_description)
- For diagrams/charts: create segments for the title, data description, and key findings
- The number of segments should match the content complexity

Return ONLY valid JSON, no markdown formatting.`

	mimeType := detectImageMimeType(imagePath)

	result, err := a.client.Models.GenerateContent(ctx, a.model, []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromBytes(imageData, mimeType),
				genai.NewPartFromText(prompt),
			},
		},
	}, &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.1)),
	})
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	text := extractText(result)
	slog.Debug("image analysis response", "text", text)

	var analysisResult ImageAnalysisResult
	if err := json.Unmarshal([]byte(cleanJSON(text)), &analysisResult); err != nil {
		return nil, fmt.Errorf("parsing image analysis response: %w\nraw: %s", err, text)
	}

	return &analysisResult, nil
}

// GenerateTitle generates a title and confidence score for text content.
func (a *Analyzer) GenerateTitle(ctx context.Context, text string) (*TitleResult, error) {
	slog.Debug("generating title", "text_length", len(text))

	// Truncate text if too long
	truncated := text
	if len(truncated) > 10000 {
		truncated = truncated[:10000]
	}

	prompt := fmt.Sprintf(`Generate a descriptive title for the following document content. Return ONLY valid JSON:
{"title": "<descriptive title>", "confidence": <0.0 to 1.0>}

Content:
%s`, truncated)

	result, err := a.client.Models.GenerateContent(ctx, a.model, []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(prompt)}},
	}, &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.1)),
	})
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	respText := extractText(result)

	var titleResult TitleResult
	if err := json.Unmarshal([]byte(cleanJSON(respText)), &titleResult); err != nil {
		return nil, fmt.Errorf("parsing title response: %w\nraw: %s", err, respText)
	}

	return &titleResult, nil
}

// GenerateSummary generates a ~100 word summary of text content.
func (a *Analyzer) GenerateSummary(ctx context.Context, text string) (string, error) {
	slog.Debug("generating summary", "text_length", len(text))

	truncated := text
	if len(truncated) > 15000 {
		truncated = truncated[:15000]
	}

	prompt := fmt.Sprintf(`Write a summary of approximately 100 words for the following document content. Return ONLY the summary text, no JSON, no markdown formatting.

Content:
%s`, truncated)

	result, err := a.client.Models.GenerateContent(ctx, a.model, []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(prompt)}},
	}, &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.3)),
	})
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}

	return extractText(result), nil
}

// DescribeSpreadsheet analyzes spreadsheet content.
func (a *Analyzer) DescribeSpreadsheet(ctx context.Context, content string) (*SpreadsheetResult, error) {
	slog.Debug("analyzing spreadsheet", "content_length", len(content))

	truncated := content
	if len(truncated) > 15000 {
		truncated = truncated[:15000]
	}

	prompt := fmt.Sprintf(`Analyze this spreadsheet content and return ONLY valid JSON:
{
  "title": "<descriptive title for this spreadsheet>",
  "confidence": <0.0 to 1.0>,
  "summary": "<~100 word summary of what this spreadsheet contains and its purpose>",
  "description": "<detailed description of the spreadsheet content, columns, data patterns, and purpose>"
}

Content:
%s`, truncated)

	result, err := a.client.Models.GenerateContent(ctx, a.model, []*genai.Content{
		{Parts: []*genai.Part{genai.NewPartFromText(prompt)}},
	}, &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.1)),
	})
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}

	respText := extractText(result)

	var spreadsheetResult SpreadsheetResult
	if err := json.Unmarshal([]byte(cleanJSON(respText)), &spreadsheetResult); err != nil {
		return nil, fmt.Errorf("parsing spreadsheet response: %w\nraw: %s", err, respText)
	}

	return &spreadsheetResult, nil
}

// extractText extracts the text content from a Gemini response.
func extractText(result *genai.GenerateContentResponse) string {
	if result == nil || len(result.Candidates) == 0 {
		return ""
	}
	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return ""
	}
	return candidate.Content.Parts[0].Text
}

// cleanJSON removes markdown code fences from JSON responses.
func cleanJSON(text string) string {
	text = removePrefix(text, "```json")
	text = removePrefix(text, "```")
	text = removeSuffix(text, "```")
	return trimSpace(text)
}

func removePrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func removeSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func trimSpace(s string) string {
	// Simple trim since we can't import strings (avoid circular imports)
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func detectImageMimeType(path string) string {
	ext := path[len(path)-4:]
	switch {
	case ext == ".jpg" || ext == "jpeg":
		return "image/jpeg"
	case ext == ".png":
		return "image/png"
	case ext == ".gif":
		return "image/gif"
	case ext == "webp":
		return "image/webp"
	case ext == "tiff" || ext == ".tif":
		return "image/tiff"
	case ext == ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}

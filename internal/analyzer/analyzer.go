package analyzer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// maxTitleInputLen is the maximum character length of text sent for title generation.
	maxTitleInputLen = 10000
	// maxSummaryInputLen is the maximum character length of text sent for summary/analysis.
	maxSummaryInputLen = 15000

	openRouterBaseURL = "https://openrouter.ai/api/v1"
)

// Analyzer performs AI-based content analysis using OpenRouter.
type Analyzer struct {
	client *openai.Client
	model  string
}

// New creates a new Analyzer configured for OpenRouter.
func New(apiKey string, model string) *Analyzer {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = openRouterBaseURL
	client := openai.NewClientWithConfig(cfg)

	return &Analyzer{
		client: client,
		model:  model,
	}
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

const maxRetries = 5

// chatCompletion wraps CreateChatCompletion with retry logic for rate limiting.
func (a *Analyzer) chatCompletion(ctx context.Context, messages []openai.ChatCompletionMessage, temperature float32) (string, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       a.model,
			Messages:    messages,
			Temperature: temperature,
		})
		if err == nil {
			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("no choices in response")
			}
			return resp.Choices[0].Message.Content, nil
		}
		if !isRetryableError(err) || attempt == maxRetries {
			return "", err
		}
		delay := time.Duration(1<<attempt) * time.Second
		slog.Warn("OpenRouter API rate limited, retrying", "attempt", attempt+1, "delay", delay)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", fmt.Errorf("unreachable")
}

func isRetryableError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "RESOURCE_EXHAUSTED") || strings.Contains(s, "503")
}

// AnalyzeImage analyzes an image file and returns structured segments.
func (a *Analyzer) AnalyzeImage(ctx context.Context, imagePath string) (*ImageAnalysisResult, error) {
	slog.Debug("analyzing image", "path", imagePath)

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("reading image file: %w", err)
	}

	mimeType := detectImageMimeType(imagePath)
	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

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

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    dataURI,
						Detail: openai.ImageURLDetailAuto,
					},
				},
				{
					Type: openai.ChatMessagePartTypeText,
					Text: prompt,
				},
			},
		},
	}

	text, err := a.chatCompletion(ctx, messages, 0.1)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}

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

	truncated := text
	if len(truncated) > maxTitleInputLen {
		truncated = truncated[:maxTitleInputLen]
	}

	prompt := fmt.Sprintf(`Generate a descriptive title for the following document content. Return ONLY valid JSON:
{"title": "<descriptive title>", "confidence": <0.0 to 1.0>}

Content:
%s`, truncated)

	respText, err := a.chatCompletion(ctx, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}, 0.1)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}

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
	if len(truncated) > maxSummaryInputLen {
		truncated = truncated[:maxSummaryInputLen]
	}

	prompt := fmt.Sprintf(`Write a summary of approximately 100 words for the following document content. Return ONLY the summary text, no JSON, no markdown formatting.

Content:
%s`, truncated)

	respText, err := a.chatCompletion(ctx, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}, 0.3)
	if err != nil {
		return "", fmt.Errorf("OpenRouter API error: %w", err)
	}

	return respText, nil
}

// DescribeSpreadsheet analyzes spreadsheet content.
func (a *Analyzer) DescribeSpreadsheet(ctx context.Context, content string) (*SpreadsheetResult, error) {
	slog.Debug("analyzing spreadsheet", "content_length", len(content))

	truncated := content
	if len(truncated) > maxSummaryInputLen {
		truncated = truncated[:maxSummaryInputLen]
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

	respText, err := a.chatCompletion(ctx, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}, 0.1)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}

	var spreadsheetResult SpreadsheetResult
	if err := json.Unmarshal([]byte(cleanJSON(respText)), &spreadsheetResult); err != nil {
		return nil, fmt.Errorf("parsing spreadsheet response: %w\nraw: %s", err, respText)
	}

	return &spreadsheetResult, nil
}

// TagRule holds a tag name and its associated auto-tagging prompt.
type TagRule struct {
	Name string
	Rule string
}

// SuggestTags asks the LLM which tags apply to a document based on tag rules.
// Only tags with non-empty rules are evaluated. Returns matching tag names.
func (a *Analyzer) SuggestTags(ctx context.Context, title string, description string, tags []TagRule) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	slog.Debug("suggesting tags", "title", title, "tag_count", len(tags))

	var tagList strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&tagList, "- %s: %s\n", t.Name, t.Rule)
	}

	prompt := fmt.Sprintf(`You are a document classifier. Given a document's title and description, determine which tags apply.

Document title: %s
Document description: %s

Available tags and their rules:
%s
For each tag, evaluate whether the document matches the tag's rule. Return ONLY a JSON array of matching tag names. If no tags match, return an empty array [].

Return ONLY valid JSON, no markdown formatting.`, title, truncateStr(description, maxSummaryInputLen), tagList.String())

	respText, err := a.chatCompletion(ctx, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}, 0.1)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API error: %w", err)
	}

	slog.Debug("tag suggestion response", "text", respText)

	var suggested []string
	if err := json.Unmarshal([]byte(cleanJSON(respText)), &suggested); err != nil {
		return nil, fmt.Errorf("parsing tag suggestion response: %w\nraw: %s", err, respText)
	}

	return suggested, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// cleanJSON removes markdown code fences from JSON responses.
func cleanJSON(text string) string {
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

func detectImageMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}

package indexer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// PDFExtractorJSON represents the JSON output from pdf-extractor.
type PDFExtractorJSON struct {
	Text   string                 `json:"text"`
	Images []PDFExtractorImage    `json:"images"`
	Pages  int                    `json:"pages"`
	Meta   map[string]interface{} `json:"meta"`
}

// PDFExtractorImage represents an extracted image from pdf-extractor JSON.
type PDFExtractorImage struct {
	Path       string `json:"image_path"`
	PageNumber int    `json:"page_number"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// parsePDFExtractorOutput parses the JSON output from pdf-extractor.
func parsePDFExtractorOutput(output string, pdfPath string) (*ExtractedPDF, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("pdf-extractor returned empty output")
	}

	// pdf-extractor may output diagnostic lines before JSON; extract the JSON object
	jsonStr := extractJSONObject(output)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in pdf-extractor output")
	}

	var pdfJSON PDFExtractorJSON
	if err := json.Unmarshal([]byte(jsonStr), &pdfJSON); err != nil {
		return nil, fmt.Errorf("parsing pdf-extractor JSON: %w", err)
	}

	result := &ExtractedPDF{
		Text: pdfJSON.Text,
	}

	pdfDir := filepath.Dir(pdfPath)
	for _, img := range pdfJSON.Images {
		imgPath := img.Path
		if !filepath.IsAbs(imgPath) {
			imgPath = filepath.Join(pdfDir, imgPath)
		}
		result.Images = append(result.Images, ExtractedImage{
			Path:       imgPath,
			PageNumber: img.PageNumber,
		})
	}

	return result, nil
}

// extractJSONObject finds the first top-level JSON object in the output string.
// This handles pdf-extractor output that may include diagnostic text before the JSON.
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

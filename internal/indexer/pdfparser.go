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
	Path       string `json:"path"`
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

	var pdfJSON PDFExtractorJSON
	if err := json.Unmarshal([]byte(output), &pdfJSON); err != nil {
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

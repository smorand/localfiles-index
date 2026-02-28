package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Client wraps Google Drive and Sheets API services.
type Client struct {
	drive  *drive.Service
	sheets *sheets.Service
}

// NewClient creates a new Google Drive client from an OAuth config and token.
func NewClient(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*Client, error) {
	tokenSource := config.TokenSource(ctx, token)
	opt := option.WithTokenSource(tokenSource)

	driveSvc, err := drive.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("creating Drive service: %w", err)
	}

	sheetsSvc, err := sheets.NewService(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("creating Sheets service: %w", err)
	}

	return &Client{drive: driveSvc, sheets: sheetsSvc}, nil
}

// GetFileInfo retrieves metadata for a Google Drive file.
func (c *Client) GetFileInfo(ctx context.Context, fileID string) (*GDriveFileInfo, error) {
	file, err := c.drive.Files.Get(fileID).
		Fields("id, name, mimeType, size, modifiedTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("getting file metadata: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, file.ModifiedTime)

	isNative := file.MimeType == MimeGoogleDoc ||
		file.MimeType == MimeGoogleSheet ||
		file.MimeType == MimeGoogleSlides

	return &GDriveFileInfo{
		ID:           file.Id,
		Name:         file.Name,
		MimeType:     file.MimeType,
		Size:         file.Size,
		ModifiedTime: modTime,
		IsNative:     isNative,
	}, nil
}

// ExportFile exports a Google native file (Docs) as Markdown to a temp file.
func (c *Client) ExportFile(ctx context.Context, fileID string, info *GDriveFileInfo) (string, error) {
	resp, err := c.drive.Files.Export(fileID, ExportMimeMarkdown).
		Context(ctx).
		Download()
	if err != nil {
		return "", fmt.Errorf("exporting file: %w", err)
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "gdrive-export-*.md")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("writing exported content: %w", err)
	}

	tmpFile.Close()
	slog.Info("exported Google Doc", "name", info.Name, "path", tmpFile.Name())
	return tmpFile.Name(), nil
}

// DownloadFile downloads a non-native Drive file to a temp file, preserving the extension.
func (c *Client) DownloadFile(ctx context.Context, fileID string, info *GDriveFileInfo) (string, error) {
	resp, err := c.drive.Files.Get(fileID).
		Context(ctx).
		Download()
	if err != nil {
		return "", fmt.Errorf("downloading file: %w", err)
	}
	defer resp.Body.Close()

	ext := filepath.Ext(info.Name)
	if ext == "" {
		ext = mimeToExt(info.MimeType)
	}

	tmpFile, err := os.CreateTemp("", "gdrive-download-*"+ext)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("writing downloaded content: %w", err)
	}

	tmpFile.Close()
	slog.Info("downloaded Drive file", "name", info.Name, "path", tmpFile.Name())
	return tmpFile.Name(), nil
}

// ReadSpreadsheet reads a Google Sheet and converts it to JSONL format.
// First row is treated as headers; each subsequent row becomes a JSON object.
func (c *Client) ReadSpreadsheet(ctx context.Context, fileID string) (string, error) {
	// Get spreadsheet metadata to find sheet names
	spreadsheet, err := c.sheets.Spreadsheets.Get(fileID).
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("getting spreadsheet metadata: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "gdrive-sheet-*.jsonl")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	for _, sheet := range spreadsheet.Sheets {
		sheetName := sheet.Properties.Title
		resp, err := c.sheets.Spreadsheets.Values.Get(fileID, sheetName).
			Context(ctx).
			Do()
		if err != nil {
			slog.Warn("failed to read sheet", "sheet", sheetName, "error", err)
			continue
		}

		if len(resp.Values) < 2 {
			continue // Need at least header + one data row
		}

		// First row = headers
		headers := make([]string, len(resp.Values[0]))
		for i, h := range resp.Values[0] {
			headers[i] = fmt.Sprintf("%v", h)
		}

		// Each subsequent row → JSON object
		for _, row := range resp.Values[1:] {
			obj := make(map[string]string)
			for i, h := range headers {
				if i < len(row) {
					obj[h] = fmt.Sprintf("%v", row[i])
				} else {
					obj[h] = ""
				}
			}
			jsonBytes, err := json.Marshal(obj)
			if err != nil {
				continue
			}
			tmpFile.Write(jsonBytes)
			tmpFile.WriteString("\n")
		}
	}

	tmpFile.Close()

	// Verify the file has content
	stat, err := os.Stat(tmpFile.Name())
	if err != nil || stat.Size() == 0 {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("spreadsheet is empty or has no data rows")
	}

	slog.Info("converted Google Sheet to JSONL", "path", tmpFile.Name())
	return tmpFile.Name(), nil
}

// mimeToExt returns a file extension for common MIME types.
func mimeToExt(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "pdf"):
		return ".pdf"
	case strings.Contains(mimeType, "png"):
		return ".png"
	case strings.Contains(mimeType, "jpeg"), strings.Contains(mimeType, "jpg"):
		return ".jpg"
	case strings.Contains(mimeType, "gif"):
		return ".gif"
	case strings.Contains(mimeType, "webp"):
		return ".webp"
	case strings.Contains(mimeType, "csv"):
		return ".csv"
	case strings.Contains(mimeType, "spreadsheetml"):
		return ".xlsx"
	case strings.Contains(mimeType, "plain"):
		return ".txt"
	case strings.Contains(mimeType, "markdown"):
		return ".md"
	default:
		return ".bin"
	}
}

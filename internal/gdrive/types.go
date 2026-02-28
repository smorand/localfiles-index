package gdrive

import (
	"strings"
	"time"
)

// GDrivePrefix is the URI scheme prefix for Google Drive files.
const GDrivePrefix = "gdrive://"

// Google MIME types for native formats.
const (
	MimeGoogleDoc    = "application/vnd.google-apps.document"
	MimeGoogleSheet  = "application/vnd.google-apps.spreadsheet"
	MimeGoogleSlides = "application/vnd.google-apps.presentation"
)

// Export MIME type for Google Docs (exported as Markdown).
const ExportMimeMarkdown = "text/markdown"

// GDriveFileInfo holds metadata about a Google Drive file.
type GDriveFileInfo struct {
	ID           string
	Name         string
	MimeType     string
	Size         int64
	ModifiedTime time.Time
	IsNative     bool // true for Google Docs/Sheets/Slides (no direct download)
}

// IsGDrivePath returns true if the path uses the gdrive:// URI scheme.
func IsGDrivePath(path string) bool {
	return strings.HasPrefix(path, GDrivePrefix)
}

// ExtractFileID extracts the file ID from a gdrive:// path.
func ExtractFileID(path string) string {
	return strings.TrimPrefix(path, GDrivePrefix)
}

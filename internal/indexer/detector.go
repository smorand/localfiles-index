package indexer

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FileType represents the detected type of a file.
type FileType string

const (
	FileTypeImage       FileType = "image"
	FileTypePDF         FileType = "pdf"
	FileTypeText        FileType = "text"
	FileTypeSpreadsheet FileType = "spreadsheet"
	FileTypeDocument    FileType = "other"
)

// FileInfo holds detected file information.
type FileInfo struct {
	Path     string
	Type     FileType
	MimeType string
	Size     int64
}

// SupportedImageExts lists supported image file extensions.
var SupportedImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".tiff": true, ".tif": true, ".bmp": true,
}

// SupportedTextExts lists supported text file extensions.
var SupportedTextExts = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".rst": true,
	".text": true, ".log": true,
}

// SupportedSpreadsheetExts lists supported spreadsheet file extensions.
var SupportedSpreadsheetExts = map[string]bool{
	".csv": true, ".xlsx": true,
}

// SupportedConvertExts lists document extensions that need conversion to PDF.
var SupportedConvertExts = map[string]bool{
	".doc": true, ".docx": true, ".odt": true,
}

// DetectFileType detects the type and MIME type of a file.
func DetectFileType(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("accessing file: %w", err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	if stat.Size() == 0 {
		return nil, fmt.Errorf("file is empty: %s", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	info := &FileInfo{
		Path: path,
		Size: stat.Size(),
	}

	// Detect MIME type from extension first
	mimeType := mime.TypeByExtension(ext)

	// If extension-based detection fails, read file header
	if mimeType == "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening file for MIME detection: %w", err)
		}
		defer f.Close()

		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		mimeType = http.DetectContentType(buf[:n])
	}

	info.MimeType = mimeType

	// Determine file type
	switch {
	case ext == ".pdf":
		info.Type = FileTypePDF
	case SupportedImageExts[ext]:
		info.Type = FileTypeImage
	case SupportedTextExts[ext]:
		info.Type = FileTypeText
	case SupportedSpreadsheetExts[ext]:
		info.Type = FileTypeSpreadsheet
	case SupportedConvertExts[ext]:
		info.Type = FileTypeDocument
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	return info, nil
}

// IsSupported returns true if the file extension is supported.
func IsSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".pdf" || SupportedImageExts[ext] || SupportedTextExts[ext] || SupportedSpreadsheetExts[ext] || SupportedConvertExts[ext]
}

package indexer

import (
	"strings"
)

// ChunkText splits text into chunks of approximately chunkSize words with overlap words between consecutive chunks.
func ChunkText(text string, chunkSize, overlap int) []ChunkResult {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []ChunkResult
	start := 0

	for start < len(words) {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}

		chunk := strings.Join(words[start:end], " ")
		chunks = append(chunks, ChunkResult{
			Content: chunk,
			Index:   len(chunks),
		})

		if end >= len(words) {
			break
		}

		start = end - overlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

// ChunkResult holds a single chunk's content and position.
type ChunkResult struct {
	Content string
	Index   int
}

package storage

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Tag represents a document tag for organizing indexed files.
type Tag struct {
	bun.BaseModel `bun:"table:tags,alias:t"`

	ID          uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Name        string    `bun:"name,notnull,unique" json:"name"`
	Description string    `bun:"description" json:"description"`
	Rule        string    `bun:"rule" json:"rule"`
	CreatedAt   time.Time `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// DocumentTag is the junction model for the many-to-many relationship between documents and tags.
type DocumentTag struct {
	bun.BaseModel `bun:"table:document_tags,alias:dt"`

	DocumentID uuid.UUID `bun:"document_id,pk,type:uuid"`
	TagID      uuid.UUID `bun:"tag_id,pk,type:uuid"`
	CreatedAt  time.Time `bun:"created_at,notnull,default:now()"`

	Document *Document `bun:"rel:belongs-to,join:document_id=id"`
	Tag      *Tag      `bun:"rel:belongs-to,join:tag_id=id"`
}

// Document represents an indexed file in the system.
type Document struct {
	bun.BaseModel `bun:"table:documents,alias:d"`

	ID              uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	FilePath        string    `bun:"file_path,notnull,unique" json:"file_path"`
	FileMtime       time.Time `bun:"file_mtime,notnull" json:"file_mtime"`
	Title           string    `bun:"title,notnull" json:"title"`
	TitleConfidence float64   `bun:"title_confidence,type:real" json:"title_confidence"`
	DocumentType    string    `bun:"document_type,notnull" json:"document_type"`
	MimeType        string    `bun:"mime_type" json:"mime_type"`
	FileSize        int64     `bun:"file_size" json:"file_size"`
	Metadata        string    `bun:"metadata,type:jsonb,default:'{}'" json:"metadata"`
	IndexedAt       time.Time `bun:"indexed_at,notnull,default:now()" json:"indexed_at"`
	CreatedAt       time.Time `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at,notnull,default:now()" json:"updated_at"`

	Tags   []*Tag   `bun:"m2m:document_tags,join:Document=Tag" json:"tags,omitempty"`
	Chunks []*Chunk `bun:"rel:has-many,join:id=document_id" json:"chunks,omitempty"`
	Images []*Image `bun:"rel:has-many,join:id=document_id" json:"images,omitempty"`
}

// Chunk represents a searchable segment of a document.
type Chunk struct {
	bun.BaseModel `bun:"table:chunks,alias:ch"`

	ID           uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	DocumentID   uuid.UUID `bun:"document_id,notnull,type:uuid" json:"document_id"`
	ChunkIndex   int       `bun:"chunk_index,notnull" json:"chunk_index"`
	Content      string    `bun:"content,notnull" json:"content"`
	ChunkType    string    `bun:"chunk_type,notnull" json:"chunk_type"`
	ChunkLabel   string    `bun:"chunk_label" json:"chunk_label"`
	SourcePage   *int      `bun:"source_page" json:"source_page"`
	Embedding    []float32 `bun:"embedding,type:vector(768)" json:"-"`
	SearchVector string    `bun:"search_vector,type:tsvector" json:"-"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:now()" json:"created_at"`

	Document *Document `bun:"rel:belongs-to,join:document_id=id" json:"-"`
}

// Image represents an extracted or standalone image associated with a document.
type Image struct {
	bun.BaseModel `bun:"table:images,alias:img"`

	ID          uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	DocumentID  uuid.UUID  `bun:"document_id,notnull,type:uuid" json:"document_id"`
	ChunkID     *uuid.UUID `bun:"chunk_id,type:uuid" json:"chunk_id"`
	ImagePath   string     `bun:"image_path,notnull" json:"image_path"`
	Description string     `bun:"description" json:"description"`
	ImageType   string     `bun:"image_type" json:"image_type"`
	Caption     string     `bun:"caption" json:"caption"`
	SourcePage  *int       `bun:"source_page" json:"source_page"`
	CreatedAt   time.Time  `bun:"created_at,notnull,default:now()" json:"created_at"`

	Document *Document `bun:"rel:belongs-to,join:document_id=id" json:"-"`
}

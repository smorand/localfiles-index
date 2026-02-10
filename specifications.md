# LocalFiles Index - Specifications

> **Version**: 1.1.0
> **Last Updated**: 2026-02-10
> **Status**: Draft
> **Owner**: Sebastien MORAND

---

## 1. Overview

**Summary**: Personal file indexing and semantic search system that extracts metadata from local files using AI and enables natural language retrieval.

**Purpose**: Enable fast, intelligent retrieval of personal documents (passports, invoices, photos, contracts, etc.) by indexing their content and metadata, supporting both natural language semantic search and full-text keyword search.

### Target Users

| User Type | Description | Primary Needs |
|-----------|-------------|---------------|
| Owner | Sebastien MORAND — personal use | Index documents, search by natural language, retrieve files and metadata quickly in structured copy-pasteable format |

---

## 2. Problem & Scope

### 2.1 Pain Points

1. **File retrieval is slow and manual**: Finding a specific document among thousands of personal files requires remembering exact file names or browsing folder hierarchies — typically 5–15 minutes per search, with frequent failures.
2. **No metadata extraction**: File content (passport details, invoice amounts, photo descriptions) is locked inside the files with no searchable index — information must be opened and read manually each time.
3. **No semantic understanding**: OS-level search only matches file names or exact text — it cannot understand "find my passport" or "show travel photos from Italy", forcing users to rely on perfect keyword recall.

### 2.2 Goals

1. **Intelligent indexing**: Automatically extract and store content and metadata from documents, images, PDFs, and spreadsheets.
2. **Natural language search**: Find any indexed document using conversational queries with semantic similarity matching.
3. **Structured results**: Return search results in a clean, structured format suitable for copy-pasting.
4. **Category-based organization**: Organize indexed files into user-defined categories (administrative, work, family photos, etc.) for filtered search.
5. **Dual interface**: Provide both a CLI for direct interaction and a network API for integration with AI tools.

### 2.3 Non-Goals

1. **Cloud deployment**: No cloud deployment, CI/CD pipeline, or DevOps automation — local-only for now.
2. **Multi-user support**: Single-user system, no user management or access control beyond API authentication.
3. **Web UI**: No graphical user interface — CLI and network API only.
4. **Real-time file watching**: No automatic detection of new/modified files — indexing and updates are triggered manually.
5. **File storage/backup**: The system indexes files but does not copy, move, or back up original files.

### 2.4 Future Considerations

- Multimodal embedding for direct image-to-image similarity search
- Google Drive integration for indexing cloud documents
- Automatic file watching with filesystem events
- Cloud deployment with remote database
- Batch import of entire directory trees with progress tracking

---

## 3. Functional Requirements

### 3.1 File Indexing

#### FR-001: Index Image Files

- **Description**: The system must accept an image file, analyze its visual content using AI with a structured prompt that instructs the model to: (1) identify the type of content (official document, photograph, diagram, etc.), (2) extract all relevant identifiable information, and (3) return a variable number of search-optimized segments tailored to the content type. The AI decides how many segments to produce and what each should contain. For example, a passport yields segments for "passport number", "holder name + nationality", and "full description"; a family photo yields only a "scene description" segment. Each segment is stored as a separate searchable chunk.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Image files (JPEG, PNG, GIF, WebP, TIFF, BMP) are accepted for indexing
  - [ ] AI analysis returns a structured response containing: content type, a title, and a list of segments (each with a label and text content)
  - [ ] The number and content of segments is determined dynamically by the AI based on the image content
  - [ ] Official documents (passport, ID card, invoice, etc.) produce segments for each key identifier (document number, holder name, dates, amounts, etc.) plus a description
  - [ ] Photographs produce fewer segments (typically just a scene description)
  - [ ] Each segment is stored as a separate independently searchable chunk
  - [ ] The file path is stored for future retrieval and update detection
  - [ ] The file modification time (mtime) is recorded for change detection

#### FR-002: Index PDF Files

- **Description**: The system must extract text and images from PDF files and produce multiple levels of indexed segments: (1) a document-level title segment, (2) a document-level summary (~100 words), (3) text segments with configurable size and overlap, and (4) for each extracted image, AI-driven segments (as per FR-001). All segments must track their source page number so that search results can indicate exactly where in the PDF the match was found.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Text is extracted from all pages of the PDF
  - [ ] A document-level title segment is generated by AI and independently searchable
  - [ ] A document-level summary segment (~100 words) is generated by AI and independently searchable
  - [ ] Text is divided into segments of a configurable word count with configurable word overlap between consecutive segments
  - [ ] Each text segment is independently searchable
  - [ ] Each text segment records the page number where it starts
  - [ ] Embedded images are extracted with the source page number preserved
  - [ ] Each extracted image is indexed with AI-driven segments (as per FR-001), each independently searchable with a label
  - [ ] Image segments record the source PDF file path and page number (source_page)
  - [ ] The original file path and file mtime are stored
  - [ ] Search results referencing a PDF segment include the file path and the page number

#### FR-003: Index Text Files

- **Description**: The system must read plain text files and produce multiple levels of indexed segments: (1) a document-level title segment, (2) a document-level summary (~100 words), and (3) text segments with configurable size and overlap, each independently searchable.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Text files (TXT, MD, and similar plain-text formats) are accepted
  - [ ] A document-level title segment is generated by AI and independently searchable
  - [ ] A document-level summary segment (~100 words) is generated by AI and independently searchable
  - [ ] Content is segmented with the same configurable parameters as PDF text
  - [ ] Each segment is independently searchable

#### FR-004: Index Spreadsheet Files

- **Description**: The system must read spreadsheet files and produce multiple levels of indexed segments: (1) a document-level title segment, (2) a document-level summary (~100 words describing the spreadsheet content and purpose), and (3) a full AI-generated description. The goal is to describe what the spreadsheet is about, not to index individual cell values.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] CSV and Excel (XLSX) files are accepted
  - [ ] A document-level title segment is generated by AI and independently searchable
  - [ ] A document-level summary segment (~100 words) is generated by AI and independently searchable
  - [ ] A full AI-generated description is stored as a separate independently searchable segment
  - [ ] All segments are independently searchable

#### FR-005: Index Other Document Formats via Intermediate Conversion

- **Description**: The system must support additional document formats (DOC, DOCX, ODT, Google Docs exports, etc.) by transparently converting them to an intermediate format for processing. The converted document is processed through the same pipeline as FR-002 (title, summary, text chunks, images), and the temporary file is discarded. The stored file path always references the **original** source file, not the intermediate file.
- **Priority**: Should Have
- **Acceptance Criteria**:
  - [ ] Supported formats are converted transparently as an internal processing step
  - [ ] The user provides only the original file path — conversion is automatic
  - [ ] Converted documents are processed identically to native PDFs (FR-002): title, summary, text chunks with page numbers, image extraction
  - [ ] The stored `file_path` is the original file path (not the temporary intermediate file)
  - [ ] Temporary files are cleaned up after processing
  - [ ] Unsupported formats produce a clear error message

#### FR-006: File Tracking and Change Detection

- **Description**: The system must store the original file path and the file's modification time (mtime) for each indexed document, enabling detection of modifications when the user requests an update.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Each indexed document records its absolute file path (unique constraint)
  - [ ] The file's modification time (mtime) is recorded at indexing time
  - [ ] The system can compare the current file mtime with the stored mtime to detect changes

#### FR-007: AI Title Generation with Confidence

- **Description**: The system must generate a descriptive title for each indexed document using AI, along with a confidence score. If the confidence is above a configurable threshold, the title is accepted automatically. Below the threshold, the user is prompted for validation through the active interface.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] A descriptive title is generated for every indexed document
  - [ ] A confidence score (0.0–1.0) accompanies each generated title
  - [ ] Titles above the configurable confidence threshold are auto-accepted
  - [ ] Titles below threshold prompt the user for confirmation or manual override
  - [ ] The confidence threshold is configurable

#### FR-008: Update All Indexed Documents

- **Description**: The system must support updating all indexed documents in a single command. It checks every indexed file's current mtime against the stored mtime, re-indexes only those that changed, and reports files that no longer exist on disk. A `--force` flag bypasses mtime comparison and re-indexes everything.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Update with no path checks all indexed documents
  - [ ] Update with a specific path checks only that file
  - [ ] Files with changed mtime are re-indexed (metadata and all indexed data replaced)
  - [ ] Files with unchanged mtime are skipped (no-op)
  - [ ] Files missing from disk are reported as warnings (not silently ignored, not auto-deleted)
  - [ ] `--force` flag forces re-indexing of all files regardless of mtime
  - [ ] Old indexed data is replaced, not appended
  - [ ] A summary is displayed: N updated, N unchanged, N missing

### 3.2 Search

#### FR-009: Semantic Search

- **Description**: The system must support natural language queries that are matched against indexed content using vector similarity. This is the default search mode.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] User provides a free-text query
  - [ ] The query is semantically compared against stored indexed content
  - [ ] Results are ranked by relevance (similarity score)
  - [ ] Results include: document title, file path, matched segment excerpt, similarity score, category, segment type, and source page number (when applicable)

#### FR-010: Full-Text Search

- **Description**: The system must support keyword-based full-text search as an alternative to semantic search, selectable via a flag.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] User can switch to full-text search mode via a flag/option
  - [ ] Full-text search matches exact keywords and word variations in indexed content
  - [ ] Results are ranked by relevance
  - [ ] Results include the same fields as semantic search

#### FR-011: Category-Filtered Search

- **Description**: The system must allow search results to be filtered by one or more categories.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] User can specify a category filter alongside any search query
  - [ ] Only documents in the specified category are included in results
  - [ ] Filter applies to both semantic and full-text search modes

#### FR-012: Structured Search Results

- **Description**: Search results must be displayed in a structured, copy-pasteable format with configurable output style.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Results support at least tabular and structured data output formats
  - [ ] A detail view shows full document information for a single result
  - [ ] The number of results is configurable with a sensible default (e.g., 10)

### 3.3 Category Management

#### FR-013: Category CRUD Operations

- **Description**: The system must support creating, listing, updating, and deleting document categories. Categories are stored persistently and referenced by documents.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Categories can be created with a name and optional description
  - [ ] All categories can be listed
  - [ ] A category's name or description can be updated
  - [ ] A category can be deleted only if no documents reference it (or with a force flag)
  - [ ] Category names are unique

#### FR-014: Assign Category During Indexing

- **Description**: The user must specify a category when indexing a file. Category is a mandatory parameter.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Category must be provided as a required parameter during indexing
  - [ ] The specified category must already exist
  - [ ] Documents can be re-assigned to a different category

### 3.4 CLI Interface

#### FR-015: CLI for All Operations

- **Description**: The system must provide a command-line interface covering all core operations: indexing, searching, category management, document viewing, deletion, server startup, and status reporting.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] All core operations are accessible as CLI subcommands
  - [ ] Each subcommand supports help documentation
  - [ ] Output is formatted for terminal readability
  - [ ] Exit codes follow conventions: 0 = success, 1 = error

### 3.5 Network API Interface

#### FR-016: Network API for Core Operations

- **Description**: The system must expose a network API that provides the same core operations as the CLI (search, index, get document, list categories, status), enabling integration with external AI tools.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] All search, indexing, category listing, and status operations are accessible via the API
  - [ ] The API can be started via a CLI subcommand
  - [ ] The API port is configurable

#### FR-017: Authenticated API Access

- **Description**: The network API must require authentication. Only clients with valid credentials can access the API.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Unauthenticated requests are rejected
  - [ ] Authentication credentials are loaded from a configurable external file
  - [ ] The authentication mechanism follows a standard protocol

### 3.6 Document Management

#### FR-018: View Document Details

- **Description**: The system must allow viewing full details of an indexed document, including title, file path, category, metadata, chunk count, and image information.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Document can be retrieved by file path or internal ID
  - [ ] All stored metadata is displayed
  - [ ] Chunk content is optionally viewable

#### FR-019: Delete Document from Index

- **Description**: The system must allow removing a document and all its associated indexed data from the index.
- **Priority**: Must Have
- **Acceptance Criteria**:
  - [ ] Document and all related records are deleted
  - [ ] Deletion by file path or internal ID
  - [ ] Confirmation prompt before deletion (CLI)

#### FR-020: Index Statistics

- **Description**: The system must display summary statistics about the index: total documents, total chunks, documents per category, storage usage.
- **Priority**: Should Have
- **Acceptance Criteria**:
  - [ ] Total document count
  - [ ] Total chunk count
  - [ ] Breakdown by document type
  - [ ] Breakdown by category

---

## 4. Non-Functional Requirements

### 4.1 Performance

#### NFR-001: Search Response Time

- **Description**: Search queries must return results in a reasonable time for a single-user local system.
- **Target**: < 2 seconds for semantic search, < 1 second for full-text search on an index of up to 100,000 chunks
- **Measurement**: Timed functional tests
- **Priority**: Should Have

#### NFR-002: Indexing Throughput

- **Description**: File indexing must be practical for batch operations.
- **Target**: A standard PDF (< 50 pages) must be fully indexed in under 30 seconds (including API calls for embedding and analysis)
- **Measurement**: Timed functional tests
- **Priority**: Should Have

### 4.2 Scalability

#### NFR-003: Index Capacity

- **Description**: The system must handle a personal document collection without degradation.
- **Target**: Support up to 10,000 documents and 100,000 chunks without performance degradation
- **Measurement**: Functional tests with representative data volume
- **Priority**: Should Have

### 4.3 Security

#### NFR-004: API Authentication

- **Description**: The network API must enforce authentication to prevent unauthorized access to the local index.
- **Target**: All API requests must be authenticated; invalid credentials result in rejection
- **Measurement**: E2E tests with valid and invalid credentials
- **Priority**: Must Have

#### NFR-005: Credential Storage

- **Description**: Sensitive credentials (API keys, OAuth secrets) must not be hardcoded; they must be loaded from environment variables or external configuration files.
- **Target**: Zero hardcoded secrets in source code
- **Measurement**: Code review, grep for patterns
- **Priority**: Must Have

### 4.4 Reliability

#### NFR-006: Data Integrity

- **Description**: All indexed data must be persisted reliably with transactional guarantees.
- **Target**: Indexing operations are atomic — partial failures do not leave inconsistent state
- **Measurement**: Integration tests with simulated failures
- **Priority**: Must Have

### 4.5 Observability

#### NFR-007: Structured Logging

- **Description**: The system must produce structured logs for all operations.
- **Target**: Structured, machine-parseable logs with timestamp, level, module, and message fields; configurable log level
- **Measurement**: Log output inspection in tests
- **Priority**: Must Have

### 4.6 Debugging

#### NFR-008: Verbose Mode

- **Description**: A verbose/debug mode must be available to trace execution flow during troubleshooting.
- **Target**: Debug log level shows: external API calls, database queries, segmentation details, embedding operations
- **Measurement**: Manual verification
- **Priority**: Should Have

### 4.7 Deployment

#### NFR-009: Local-Only Deployment

- **Description**: The system runs locally on the developer's machine. No cloud deployment or CI/CD is required for this version.
- **Target**: A single build-and-install process produces a working executable
- **Measurement**: Build and install on the target platform
- **Priority**: Must Have

### 4.8 Resilience

#### NFR-010: External API Failure Handling

- **Description**: Failures from external AI APIs must not crash the system; they must be handled gracefully with informative error messages.
- **Target**: Transient API errors are retried (up to 3 attempts with exponential backoff); permanent failures log the error and skip the file with a clear message
- **Measurement**: Integration tests with simulated API failures
- **Priority**: Must Have

### 4.9 Maintainability

#### NFR-011: Code Quality

- **Description**: Code must follow language-idiomatic conventions, pass static analysis and formatting checks, and have adequate test coverage on core logic.
- **Target**: Pass all static analysis and formatting checks; integration + E2E test coverage on all Must Have FRs
- **Measurement**: Automated quality checks pass; test coverage report
- **Priority**: Must Have

### 4.10 Compatibility

#### NFR-012: Platform Support

- **Description**: The system must run on macOS (primary development platform).
- **Target**: Full functionality on macOS (darwin/arm64 and darwin/amd64)
- **Measurement**: Build and functional tests on macOS
- **Priority**: Must Have

### 4.11 Data Governance

#### NFR-013: Index Data Lifecycle

- **Description**: The index must remain consistent with the filesystem. Orphaned records (documents whose source files no longer exist) should be detectable and cleanable.
- **Target**: A maintenance operation can identify and optionally remove orphaned records; no automatic purge without user confirmation
- **Measurement**: Functional test: delete a source file, run maintenance, verify orphan is detected
- **Priority**: Should Have

### 4.12 Cost

#### NFR-014: API Cost Efficiency

- **Description**: External AI API usage should be minimized where practical.
- **Target**: Embeddings and AI analysis are only called when content has changed (mtime check); no redundant API calls on update of unchanged files
- **Measurement**: Verify no API calls during no-op re-index
- **Priority**: Should Have

### 4.13 Internationalization

#### NFR-015: Multilingual Content Support

- **Description**: The system must handle documents in multiple languages (primarily French and English).
- **Target**: Indexing and semantic search work correctly on French and English content; embedding model supports multilingual input
- **Measurement**: Functional tests with French and English documents
- **Priority**: Must Have

---

## 5. Tests

### 5.1 Coverage Summary

| Requirement | Happy Path TS | Error Path TS | Status |
|-------------|---------------|---------------|--------|
| FR-001 | TS-001, TS-034, TS-035 | TS-025 | Covered |
| FR-002 | TS-002, TS-003, TS-036 | TS-026 | Covered |
| FR-003 | TS-004 | TS-051 | Covered |
| FR-004 | TS-005, TS-037 | TS-052 | Covered |
| FR-005 | TS-006 | TS-022 | Covered |
| FR-006 | TS-007, TS-012 | TS-024 | Covered |
| FR-007 | TS-008 | TS-033 | Covered |
| FR-008 | TS-012, TS-012b, TS-012c, TS-044, TS-049 | TS-024 | Covered |
| FR-009 | TS-009 | TS-023 | Covered |
| FR-010 | TS-010 | TS-023 | Covered |
| FR-011 | TS-011 | TS-029 | Covered |
| FR-012 | TS-009, TS-010, TS-036, TS-040 | TS-023 | Covered |
| FR-013 | TS-013, TS-042 | TS-028 | Covered |
| FR-014 | TS-001, TS-002, TS-038, TS-048 | TS-027 | Covered |
| FR-015 | TS-014, TS-039, TS-040, TS-041, TS-042, TS-043, TS-044, TS-045 | TS-021 | Covered |
| FR-016 | TS-016, TS-017, TS-046, TS-047, TS-048, TS-049 | TS-032, TS-050 | Covered |
| FR-017 | TS-015 | TS-018 | Covered |
| FR-018 | TS-014, TS-041 | TS-030 | Covered |
| FR-019 | TS-014 | TS-031 | Covered |
| FR-020 | TS-014, TS-043 | — | ⚠️ No error path (read-only, always succeeds) |
| NFR-004 | TS-015 | TS-018 | Covered |
| NFR-006 | TS-012 | TS-025, TS-026 | Covered |
| NFR-007 | TS-014, TS-045 | — | Covered |
| NFR-008 | TS-045 | — | Covered |
| NFR-010 | TS-019 | — | Covered |
| NFR-015 | TS-020 | — | Covered |

### 5.2 Integration Tests

#### TS-001: Index an Image File

- **Description**: Index a JPEG image of a document (e.g., passport) and verify all metadata is extracted and stored.
- **Type**: Automated
- **Preconditions**: PostgreSQL running with schema migrated; Gemini API key configured; a test image file available; a category "administratif" exists
- **Steps**:
  1. Run index command on a sample image with category "administratif"
  2. Query the database for the document record
  3. Query the database for associated chunks and images
- **Expected Results**:
  - [ ] Document record exists with correct file_path, file_mtime, document_type="image"
  - [ ] A title is generated with a confidence score
  - [ ] At least 1 chunk exists with chunk_type="image_segment", each with a chunk_label and non-null embedding
  - [ ] Official document images produce multiple segments (identifiers + description); photographs produce fewer
  - [ ] An image record exists with description, type, and caption populated
  - [ ] Category is set to "administratif"
- **Validates**: FR-001, FR-014
- **Priority**: Critical

#### TS-002: Index a PDF File

- **Description**: Index a multi-page PDF with text and embedded images, and verify correct chunking and image extraction.
- **Type**: Automated
- **Preconditions**: PostgreSQL running; Gemini API key configured; pdf-extractor available; a test PDF (>2 pages with at least 1 image) available; a category exists
- **Steps**:
  1. Run index command on the test PDF with a category
  2. Query the database for the document record
  3. Query the database for chunks (count and content)
  4. Query the database for images
- **Expected Results**:
  - [ ] Document record exists with document_type="pdf"
  - [ ] One chunk with chunk_type="doc_title" exists with a non-null embedding
  - [ ] One chunk with chunk_type="doc_summary" exists (~100 words) with a non-null embedding
  - [ ] Text chunks exist with correct 100-word size and 5-word overlap, each with source_page set
  - [ ] Each text chunk has a non-null embedding
  - [ ] Image records exist with AI-generated descriptions and source_page set
  - [ ] Each image has at least 1 associated chunk (chunk_type="image_segment") with embedding, chunk_label, and source_page
- **Validates**: FR-002, FR-014
- **Priority**: Critical

#### TS-003: PDF Chunking Accuracy

- **Description**: Verify that text chunking produces correct chunk sizes and overlaps.
- **Type**: Automated
- **Preconditions**: A text content of known word count
- **Steps**:
  1. Chunk a text of 250 words with chunk_size=100 and overlap=5
  2. Inspect the resulting chunks
- **Expected Results**:
  - [ ] 3 chunks are produced
  - [ ] Chunk 1: words 1–100, Chunk 2: words 96–195 (5-word overlap), Chunk 3: words 191–250
  - [ ] Each chunk boundary preserves word integrity (no split words)
- **Validates**: FR-002
- **Priority**: Critical

#### TS-004: Index a Text File

- **Description**: Index a plain text file and verify chunking and embedding.
- **Type**: Automated
- **Preconditions**: PostgreSQL running; a test .txt file with >100 words
- **Steps**:
  1. Run index command on the text file with a category
  2. Query database for document and chunks
- **Expected Results**:
  - [ ] Document record exists with document_type="text"
  - [ ] Chunks are created with correct word count and overlap
  - [ ] Each chunk has a non-null embedding
- **Validates**: FR-003
- **Priority**: High

#### TS-005: Index a Spreadsheet File

- **Description**: Index a CSV file and verify that an AI-generated description is stored.
- **Type**: Automated
- **Preconditions**: PostgreSQL running; Gemini API key configured; a test CSV file
- **Steps**:
  1. Run index command on the CSV file with a category
  2. Query database for document and chunks
- **Expected Results**:
  - [ ] Document record exists with document_type="spreadsheet"
  - [ ] At least one chunk contains an AI-generated description of the spreadsheet content
  - [ ] The chunk has a non-null embedding
- **Validates**: FR-004
- **Priority**: High

#### TS-006: Index a DOCX File via PDF Conversion

- **Description**: Index a DOCX file, verifying it is converted to PDF and processed correctly.
- **Type**: Automated
- **Preconditions**: LibreOffice installed; pdf-extractor available; a test .docx file
- **Steps**:
  1. Run index command on the DOCX file with a category
  2. Query database for document and chunks
- **Expected Results**:
  - [ ] Document record exists with the original .docx file_path
  - [ ] Text chunks are created (same as PDF processing)
  - [ ] Conversion is transparent to the user
- **Validates**: FR-005
- **Priority**: Medium

#### TS-007: File Mtime Storage and Change Detection

- **Description**: Verify that indexing stores the file's mtime and that modification is detected when the file changes.
- **Type**: Automated
- **Preconditions**: A test file indexed once
- **Steps**:
  1. Index a test file and record the stored mtime
  2. Modify the file content (which updates its mtime)
  3. Query the stored mtime from the database
  4. Compare with the file's current mtime
- **Expected Results**:
  - [ ] Initial mtime is stored in the document record
  - [ ] Modified file has a newer mtime than the stored value
- **Validates**: FR-006
- **Priority**: High

#### TS-008: Title Generation with Confidence Scoring

- **Description**: Verify that AI-generated titles include confidence scores and that the threshold mechanism works.
- **Type**: Automated
- **Preconditions**: Gemini API configured; test files of varying complexity
- **Steps**:
  1. Index a clear, well-defined document (e.g., a passport image)
  2. Check the generated title and confidence score
  3. Verify the title was auto-accepted (above threshold)
- **Expected Results**:
  - [ ] Title is descriptive and relevant to the document content
  - [ ] Confidence score is between 0.0 and 1.0
  - [ ] Title above threshold is stored without user prompt
- **Validates**: FR-007
- **Priority**: High

#### TS-009: Semantic Search Returns Relevant Results

- **Description**: Index multiple documents of different types and verify that a natural language query returns the most relevant one.
- **Type**: Automated
- **Preconditions**: At least 3 documents indexed (1 passport image, 1 invoice PDF, 1 text file)
- **Steps**:
  1. Search for "passport Sébastien Morand" using semantic mode
  2. Inspect the results
- **Expected Results**:
  - [ ] The passport image document appears as the top result
  - [ ] Results include title, file path, excerpt, and similarity score
  - [ ] Results are ordered by descending similarity score
- **Validates**: FR-009, FR-012
- **Priority**: Critical

#### TS-010: Full-Text Search Returns Matching Results

- **Description**: Verify that full-text search mode matches exact keywords.
- **Type**: Automated
- **Preconditions**: Documents indexed with known text content
- **Steps**:
  1. Search for a specific keyword known to exist in one document, using full-text mode
  2. Inspect the results
- **Expected Results**:
  - [ ] The document containing the keyword appears in results
  - [ ] Documents not containing the keyword do not appear
- **Validates**: FR-010, FR-012
- **Priority**: Critical

#### TS-011: Category-Filtered Search

- **Description**: Verify that category filtering restricts search results.
- **Type**: Automated
- **Preconditions**: Documents indexed in different categories
- **Steps**:
  1. Search with a broad query and filter by category "administratif"
  2. Inspect results
- **Expected Results**:
  - [ ] Only documents in category "administratif" are returned
  - [ ] Documents in other categories are excluded
- **Validates**: FR-011
- **Priority**: High

#### TS-012: Re-index Modified File

- **Description**: Modify an indexed file and verify the update command re-indexes it correctly.
- **Type**: Automated
- **Preconditions**: A file already indexed
- **Steps**:
  1. Modify the file content (which updates its mtime)
  2. Run `update` command on the file
  3. Query database for updated records
- **Expected Results**:
  - [ ] Stored mtime is updated to the new value
  - [ ] Old chunks are replaced with new ones
  - [ ] No duplicate document records
  - [ ] New embeddings reflect the updated content
- **Validates**: FR-006, FR-008, NFR-006
- **Priority**: High

#### TS-012b: Update All Documents

- **Description**: Verify the update command (without path) checks all indexed documents and correctly identifies changed, unchanged, and missing files.
- **Type**: Automated
- **Preconditions**: 3 files indexed: file_A (unchanged), file_B (will be modified), file_C (will be deleted from disk)
- **Steps**:
  1. Modify file_B content
  2. Delete file_C from disk
  3. Run `update` command with no path
  4. Check output summary and database state
- **Expected Results**:
  - [ ] file_A is skipped (unchanged mtime)
  - [ ] file_B is re-indexed (new chunks, updated mtime)
  - [ ] file_C is reported as missing (warning in output)
  - [ ] Summary displays: "1 updated, 1 unchanged, 1 missing"
- **Validates**: FR-008
- **Priority**: High

#### TS-012c: Update with --force Flag

- **Description**: Verify the `--force` flag re-indexes all documents regardless of mtime.
- **Type**: Automated
- **Preconditions**: 2 files indexed, neither modified
- **Steps**:
  1. Run `update --force` command
  2. Check that both files are re-indexed
- **Expected Results**:
  - [ ] Both files are re-indexed despite unchanged mtime
  - [ ] Chunks and embeddings are regenerated
  - [ ] Summary displays: "2 updated, 0 unchanged, 0 missing"
- **Validates**: FR-008
- **Priority**: Medium

#### TS-034: Index a PNG Image

- **Description**: Verify that non-JPEG image formats are correctly indexed.
- **Type**: Automated
- **Preconditions**: PostgreSQL running; Gemini API key configured; a test PNG image (e.g., a screenshot or diagram)
- **Steps**:
  1. Run index command on a PNG file with a category
  2. Query database for document and chunks
- **Expected Results**:
  - [ ] Document record exists with document_type="image" and correct mime_type
  - [ ] AI-driven segments are created with embeddings
- **Validates**: FR-001
- **Priority**: Medium

#### TS-035: Image Segment Variability

- **Description**: Verify that the AI produces different numbers of segments based on content type (official document vs photograph).
- **Type**: Automated
- **Preconditions**: Gemini API configured; a passport/ID image (fixture: `official_document.jpg`) and a family photo (fixture: `family_photo.jpg`)
- **Steps**:
  1. Index the official document image
  2. Index the family photo image
  3. Query chunk count and labels for each
- **Expected Results**:
  - [ ] Official document produces 3+ image_segment chunks with distinct labels (e.g., "document_number", "holder_name", "description")
  - [ ] Family photo produces 1–2 image_segment chunks (e.g., "scene_description")
  - [ ] Each chunk has a non-null embedding and chunk_label
- **Validates**: FR-001
- **Priority**: High

#### TS-036: PDF Search Results Include Page Number

- **Description**: Verify that search results for a PDF chunk include the source page number.
- **Type**: Automated
- **Preconditions**: A multi-page PDF indexed with known content on page 2
- **Steps**:
  1. Search for a term known to appear only on page 2
  2. Inspect the search result
- **Expected Results**:
  - [ ] The matching result includes `source_page: 2`
  - [ ] The result includes the file path of the PDF
- **Validates**: FR-002, FR-012
- **Priority**: High

#### TS-037: Index an XLSX Spreadsheet File

- **Description**: Verify that XLSX files are accepted and processed like CSV files.
- **Type**: Automated
- **Preconditions**: PostgreSQL running; Gemini API configured; a test XLSX file (fixture: `sample.xlsx`)
- **Steps**:
  1. Run index command on the XLSX file with a category
  2. Query database for document and chunks
- **Expected Results**:
  - [ ] Document record exists with document_type="spreadsheet"
  - [ ] A doc_title and doc_summary chunk exist with embeddings
  - [ ] A description chunk exists with non-null embedding
- **Validates**: FR-004
- **Priority**: Medium

#### TS-038: Category Reassignment

- **Description**: Verify that a document can be reassigned from one category to another.
- **Type**: Automated
- **Preconditions**: A document indexed with category "travail"; a category "administratif" exists
- **Steps**:
  1. Re-index the same file with `--category administratif`
  2. Query the document record
- **Expected Results**:
  - [ ] Document category_id now references "administratif"
  - [ ] No duplicate document records
  - [ ] Chunks and embeddings remain intact (not re-generated if content unchanged)
- **Validates**: FR-014
- **Priority**: Medium

### 5.3 E2E Tests — CLI

#### TS-013: Category CRUD via CLI

- **Description**: Full category lifecycle through the CLI.
- **Type**: Automated
- **Preconditions**: Empty database with schema migrated
- **Steps**:
  1. Run `categories add administratif "Documents administratifs"`
  2. Run `categories list` and verify "administratif" appears
  3. Run `categories update administratif --description "Documents admin et officiels"`
  4. Run `categories list` and verify updated description
  5. Run `categories remove administratif` (should succeed if no documents reference it)
  6. Run `categories list` and verify "administratif" is gone
- **Expected Results**:
  - [ ] Each operation succeeds with exit code 0
  - [ ] List output correctly reflects current state after each operation
- **Validates**: FR-013
- **Priority**: High

#### TS-014: Full CLI Workflow

- **Description**: End-to-end CLI workflow: create category, index file, search, show details, check status, delete.
- **Type**: Automated
- **Preconditions**: PostgreSQL running; Gemini API key configured; test files available
- **Steps**:
  1. Run `categories add test "Test category"`
  2. Run `index /path/to/test-image.jpg --category test`
  3. Run `search "test image content"` and verify result
  4. Run `show <document-id>` and verify all fields
  5. Run `status` and verify counts (1 document)
  6. Run `delete <document-id>` and confirm
  7. Run `status` and verify counts (0 documents)
- **Expected Results**:
  - [ ] Each command produces correct output
  - [ ] All exit codes are 0
  - [ ] Structured log output is emitted at configured level
- **Validates**: FR-015, FR-018, FR-019, FR-020, NFR-007
- **Priority**: Critical

#### TS-039: Automatic Directory Indexing via CLI

- **Description**: Verify that passing a directory path automatically indexes all supported files in the directory tree (recursive by default).
- **Type**: Automated
- **Preconditions**: A directory with 2 supported files (image + text) and 1 unsupported file (.zip), with a subdirectory containing 1 more supported file
- **Steps**:
  1. Run `index /path/to/dir --category test`
  2. Run `status` to check document count
  3. Query database for documents
- **Expected Results**:
  - [ ] 3 documents are indexed (2 in root dir + 1 in subdirectory)
  - [ ] The unsupported file is skipped with a warning (not an error)
  - [ ] All 3 documents have category "test"
  - [ ] Exit code 0
- **Validates**: FR-015
- **Priority**: High

#### TS-040: Search Output Formats via CLI

- **Description**: Verify all search output formats: table, json, detail.
- **Type**: Automated
- **Preconditions**: At least 2 documents indexed
- **Steps**:
  1. Run `search "test query" --format table`
  2. Run `search "test query" --format json`
  3. Run `search "test query" --format detail`
  4. Run `search "test query" --limit 1`
- **Expected Results**:
  - [ ] Table format: columnar output with headers (title, path, score)
  - [ ] JSON format: valid JSON array parseable by `jq`
  - [ ] Detail format: full document information for each result
  - [ ] Limit flag: only 1 result returned
  - [ ] All exit codes are 0
- **Validates**: FR-012, FR-015
- **Priority**: Medium

#### TS-041: Show Document with Chunks Flag

- **Description**: Verify the `--chunks` flag displays chunk content along with document details.
- **Type**: Automated
- **Preconditions**: A PDF document indexed (with multiple chunks)
- **Steps**:
  1. Run `show <path>` (without chunks flag)
  2. Run `show <path> --chunks`
- **Expected Results**:
  - [ ] Without flag: shows document metadata only (title, path, type, category, chunk count)
  - [ ] With flag: shows document metadata AND content of each chunk with chunk_type, chunk_label, source_page
- **Validates**: FR-018, FR-015
- **Priority**: Medium

#### TS-042: Category Remove with Force Flag

- **Description**: Verify that `categories remove --force` deletes a category even when documents reference it.
- **Type**: Automated
- **Preconditions**: Category "temp" exists with 1 document referencing it
- **Steps**:
  1. Run `categories remove temp` (without force) — should fail
  2. Run `categories remove temp --force` — should succeed
  3. Query the document that referenced "temp"
- **Expected Results**:
  - [ ] Step 1: exit code 1 with "category has documents" error
  - [ ] Step 2: exit code 0, category deleted
  - [ ] Step 3: document still exists, category_id is null
- **Validates**: FR-013, FR-015
- **Priority**: Medium

#### TS-043: Status JSON Output

- **Description**: Verify that `status --format json` outputs valid machine-readable JSON.
- **Type**: Automated
- **Preconditions**: At least 1 document indexed in a category
- **Steps**:
  1. Run `status --format json`
  2. Parse output as JSON
- **Expected Results**:
  - [ ] Output is valid JSON parseable by `jq`
  - [ ] Contains: total_documents, total_chunks, by_type (map), by_category (map)
- **Validates**: FR-020, FR-015
- **Priority**: Low

#### TS-044: CLI Update Subcommand E2E

- **Description**: Full E2E test of the update subcommand through the CLI: specific file, all files, and force.
- **Type**: Automated
- **Preconditions**: 2 files indexed; file_A modified, file_B unchanged
- **Steps**:
  1. Run `update /path/to/file_A` — update specific file
  2. Verify file_A is re-indexed (exit code 0, output says "1 updated")
  3. Run `update` — update all
  4. Verify output: "0 updated, 2 unchanged, 0 missing"
  5. Run `update --force` — force re-index all
  6. Verify output: "2 updated, 0 unchanged, 0 missing"
- **Expected Results**:
  - [ ] Specific file update re-indexes only that file
  - [ ] Update all with no changes is a no-op
  - [ ] Force flag re-indexes everything
  - [ ] All exit codes are 0
- **Validates**: FR-008, FR-015
- **Priority**: High

#### TS-045: Verbose Mode

- **Description**: Verify that the `--verbose` flag enables debug-level logging.
- **Type**: Automated
- **Preconditions**: A test file available for indexing
- **Steps**:
  1. Run `index /path/to/test.jpg --category test --verbose`
  2. Capture stderr output
- **Expected Results**:
  - [ ] Debug-level log lines appear in stderr (e.g., API request details, embedding generation, DB queries)
  - [ ] Normal operation without `--verbose` does not show debug logs
- **Validates**: NFR-007, NFR-008
- **Priority**: Low

### 5.4 E2E Tests — MCP HTTP

#### TS-015: OAuth Authentication — Valid Credentials

- **Description**: Verify that the MCP HTTP server accepts valid OAuth credentials and issues an access token.
- **Type**: Automated
- **Preconditions**: MCP server running with OAuth credentials configured
- **Steps**:
  1. Request a token from the server's token endpoint with valid client_id and client_secret
  2. Use the token to call an MCP tool
- **Expected Results**:
  - [ ] Token is issued successfully
  - [ ] MCP tool call succeeds with valid token
- **Validates**: FR-017, NFR-004
- **Priority**: Critical

#### TS-016: MCP Search Tool

- **Description**: Verify the search tool works via MCP HTTP.
- **Type**: Automated
- **Preconditions**: MCP server running; documents indexed; valid authentication token
- **Steps**:
  1. Call the MCP `search` tool with a query via HTTP
  2. Inspect the tool response
- **Expected Results**:
  - [ ] Response contains matching documents with title, path, and score
  - [ ] Response follows MCP tool result format
- **Validates**: FR-016
- **Priority**: Critical

#### TS-017: MCP Index Tool

- **Description**: Verify the index tool works via MCP HTTP.
- **Type**: Automated
- **Preconditions**: MCP server running; valid authentication token; test file accessible
- **Steps**:
  1. Call the MCP `index` tool with a file path via HTTP
  2. Verify the file is indexed by calling the MCP `search` tool
- **Expected Results**:
  - [ ] Index operation succeeds
  - [ ] File is searchable via MCP search
- **Validates**: FR-016
- **Priority**: High

#### TS-018: Unauthorized MCP Request

- **Description**: Verify that unauthenticated requests to the MCP server are rejected.
- **Type**: Automated
- **Preconditions**: MCP server running
- **Steps**:
  1. Call an MCP tool without an authentication token
  2. Call an MCP tool with an invalid token
- **Expected Results**:
  - [ ] Both requests are rejected with appropriate error
  - [ ] No data is returned
- **Validates**: FR-017, NFR-004
- **Priority**: Critical

#### TS-046: MCP Full Workflow

- **Description**: Complete MCP workflow exercising all tools in sequence: create category, index file, search, get document, update, status, delete.
- **Type**: Automated
- **Preconditions**: MCP server running; valid OAuth token; test image file accessible
- **Steps**:
  1. Call `list_categories` — verify empty or known state
  2. Call `index_file` with path and category
  3. Call `search` with a query matching the indexed file
  4. Call `get_document` with the file path from step 2
  5. Call `status` — verify document count includes the new file
  6. Call `update` with no path (check all)
  7. Call `delete_document` with the file path
  8. Call `search` again — verify file no longer returned
- **Expected Results**:
  - [ ] Each tool returns a valid MCP tool result
  - [ ] `index_file` returns success with document ID
  - [ ] `search` returns the indexed file as a match
  - [ ] `get_document` returns full document details (title, type, chunks, category)
  - [ ] `status` reflects accurate counts
  - [ ] `update` returns summary (0 updated if no changes)
  - [ ] `delete_document` returns success
  - [ ] Post-delete search returns no match for the file
- **Validates**: FR-016
- **Priority**: Critical

#### TS-047: MCP Search with All Parameters

- **Description**: Verify the MCP `search` tool with category filter, fulltext mode, and limit.
- **Type**: Automated
- **Preconditions**: MCP server running; 3 documents indexed (2 in category "admin", 1 in "work"); valid OAuth token
- **Steps**:
  1. Call `search` with `query`, `category: "admin"` — verify only admin docs returned
  2. Call `search` with `query`, `mode: "fulltext"` — verify fulltext results
  3. Call `search` with `query`, `limit: 1` — verify only 1 result
- **Expected Results**:
  - [ ] Category filter restricts results to matching category only
  - [ ] Fulltext mode returns keyword-matched results
  - [ ] Limit parameter caps number of results
- **Validates**: FR-016
- **Priority**: High

#### TS-048: MCP Index with Category

- **Description**: Verify the MCP `index_file` tool with the optional category parameter.
- **Type**: Automated
- **Preconditions**: MCP server running; valid OAuth token; category "test" exists
- **Steps**:
  1. Call `index_file` with `path` and `category: "test"`
  2. Call `get_document` for the indexed file
- **Expected Results**:
  - [ ] Document is indexed with category "test"
  - [ ] `get_document` confirms category assignment
- **Validates**: FR-016
- **Priority**: Medium

#### TS-049: MCP Update Tool

- **Description**: Verify the MCP `update` tool with and without force parameter.
- **Type**: Automated
- **Preconditions**: MCP server running; 2 documents indexed; 1 file modified on disk
- **Steps**:
  1. Call `update` with no parameters — check all documents
  2. Verify response shows "1 updated, 1 unchanged"
  3. Call `update` with `force: true`
  4. Verify response shows "2 updated"
- **Expected Results**:
  - [ ] Update without force only re-indexes changed files
  - [ ] Update with force re-indexes all files
  - [ ] Response includes update summary
- **Validates**: FR-016, FR-008
- **Priority**: High

#### TS-050: MCP Tool Error Paths

- **Description**: Verify error handling for each MCP tool when given invalid inputs.
- **Type**: Automated
- **Preconditions**: MCP server running; valid OAuth token
- **Steps**:
  1. Call `index_file` with a non-existent file path and a valid category
  2. Call `index_file` with a non-existent category
  3. Call `get_document` with a non-existent path
  4. Call `get_document` with a non-existent ID
  5. Call `delete_document` with a non-existent path
  6. Call `search` with `category` that does not exist
  7. Call `update` with a non-existent file path
- **Expected Results**:
  - [ ] All return appropriate MCP error responses (isError: true)
  - [ ] Error messages are descriptive (file not found, category not found, document not found)
  - [ ] Server does not crash on any error
- **Validates**: FR-016
- **Priority**: High

#### TS-051: Index Unreadable Text File

- **Description**: Verify proper error when indexing a text file that cannot be read (e.g., permission denied or binary content with .txt extension).
- **Type**: Automated
- **Preconditions**: A .txt file with no read permission (chmod 000) or a binary file renamed to .txt
- **Steps**:
  1. Run index command on the unreadable text file with a valid category
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Clear error message indicating the file cannot be read or parsed
  - [ ] No partial data left in database
- **Validates**: FR-003
- **Priority**: Medium

#### TS-052: Index Corrupt Spreadsheet File

- **Description**: Verify proper error when indexing a corrupt or invalid spreadsheet file.
- **Type**: Automated
- **Preconditions**: A file with .csv extension containing invalid/unparseable content (e.g., random binary data renamed to .csv) and a corrupt .xlsx file
- **Steps**:
  1. Run index command on the corrupt CSV file with a valid category
  2. Run index command on the corrupt XLSX file with a valid category
- **Expected Results**:
  - [ ] Both return exit code 1
  - [ ] Clear error message for each case
  - [ ] No partial data left in database
- **Validates**: FR-004
- **Priority**: Medium

### 5.5 Error & Edge Case Scenarios

#### TS-019: External API Failure During Indexing

- **Description**: Verify graceful handling when the AI API is unavailable during indexing.
- **Type**: Automated
- **Preconditions**: Invalid or unreachable API key configured
- **Steps**:
  1. Attempt to index a file with API unavailable
  2. Observe error output
- **Expected Results**:
  - [ ] System does not crash
  - [ ] Clear error message is logged indicating API failure
  - [ ] No partial/corrupt data is left in the database
- **Validates**: NFR-010
- **Priority**: High

#### TS-020: Multilingual Content Indexing and Search

- **Description**: Verify that French and English documents are correctly indexed and searchable.
- **Type**: Automated
- **Preconditions**: Test files in French and English
- **Steps**:
  1. Index a French document and an English document
  2. Search with a French query
  3. Search with an English query
- **Expected Results**:
  - [ ] French query finds the French document
  - [ ] English query finds the English document
  - [ ] Cross-language semantic search finds relevant documents regardless of query language
- **Validates**: NFR-015
- **Priority**: High

#### TS-021: Index Non-Existent File

- **Description**: Verify proper error when indexing a file that does not exist.
- **Type**: Automated
- **Preconditions**: A category exists
- **Steps**:
  1. Run index command with a non-existent file path and a valid category
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Clear error message: "file not found"
- **Validates**: FR-015
- **Priority**: Medium

#### TS-022: Index Unsupported File Type

- **Description**: Verify proper error when indexing an unsupported file format.
- **Type**: Automated
- **Preconditions**: A binary file (e.g., .exe, .zip); a category exists
- **Steps**:
  1. Run index command on the unsupported file with a valid category
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Clear error message indicating unsupported format
- **Validates**: FR-005
- **Priority**: Medium

#### TS-023: Search on Empty Index

- **Description**: Verify search returns empty results gracefully on an empty index.
- **Type**: Automated
- **Preconditions**: Empty database
- **Steps**:
  1. Run search with any query
- **Expected Results**:
  - [ ] Exit code 0
  - [ ] Empty result set (no error)
- **Validates**: FR-009, FR-010
- **Priority**: Medium

#### TS-024: Duplicate File Indexing

- **Description**: Verify that indexing the same file twice does not create duplicates.
- **Type**: Automated
- **Preconditions**: A file already indexed with a category
- **Steps**:
  1. Run index command on the same file again with the same category
- **Expected Results**:
  - [ ] No duplicate document record
  - [ ] Existing record is updated (or no-op if unchanged)
- **Validates**: FR-006, FR-008
- **Priority**: High

#### TS-025: Index Corrupt Image File

- **Description**: Verify proper error when indexing a corrupt or zero-byte image file.
- **Type**: Automated
- **Preconditions**: A zero-byte .jpg file or a renamed binary file with .jpg extension
- **Steps**:
  1. Run index command on the corrupt image with a valid category
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Clear error message indicating invalid image
  - [ ] No partial data left in database
- **Validates**: FR-001
- **Priority**: Medium

#### TS-026: Index Corrupt PDF File

- **Description**: Verify proper error when indexing a corrupt or password-protected PDF.
- **Type**: Automated
- **Preconditions**: A corrupt PDF file and a password-protected PDF file
- **Steps**:
  1. Run index command on the corrupt PDF with a valid category
  2. Run index command on the password-protected PDF with a valid category
- **Expected Results**:
  - [ ] Both return exit code 1
  - [ ] Clear error message for each case
  - [ ] No partial data left in database
- **Validates**: FR-002
- **Priority**: Medium

#### TS-027: Index with Non-Existent Category

- **Description**: Verify proper error when specifying a category that does not exist during indexing.
- **Type**: Automated
- **Preconditions**: No category named "nonexistent" exists
- **Steps**:
  1. Run index command with `--category nonexistent`
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Error message: category not found
  - [ ] File is not indexed
- **Validates**: FR-014
- **Priority**: High

#### TS-028: Category CRUD Error Paths

- **Description**: Verify error handling for category operations: duplicate creation, delete with references, update non-existent.
- **Type**: Automated
- **Preconditions**: Category "test" exists with at least one document referencing it
- **Steps**:
  1. Run `categories add test` (duplicate)
  2. Run `categories remove test` (referenced by documents, without force)
  3. Run `categories update nonexistent --description "x"`
- **Expected Results**:
  - [ ] Step 1: exit code 1 with "already exists" error
  - [ ] Step 2: exit code 1 with "category has documents" error
  - [ ] Step 3: exit code 1 with "not found" error
- **Validates**: FR-013
- **Priority**: High

#### TS-029: Search with Non-Existent Category Filter

- **Description**: Verify proper handling when filtering search by a category that does not exist.
- **Type**: Automated
- **Preconditions**: Indexed documents exist
- **Steps**:
  1. Search with `--category nonexistent`
- **Expected Results**:
  - [ ] Exit code 1 with "category not found" error
- **Validates**: FR-011
- **Priority**: Medium

#### TS-030: View Non-Existent Document

- **Description**: Verify proper error when viewing a document that does not exist.
- **Type**: Automated
- **Preconditions**: Empty or populated database without the queried ID
- **Steps**:
  1. Run `show nonexistent-uuid`
  2. Run `show /nonexistent/path.txt`
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Clear "document not found" error message
- **Validates**: FR-018
- **Priority**: Medium

#### TS-031: Delete Non-Existent Document

- **Description**: Verify proper error when deleting a document that does not exist.
- **Type**: Automated
- **Preconditions**: No document with the queried ID
- **Steps**:
  1. Run `delete nonexistent-uuid`
- **Expected Results**:
  - [ ] Exit code 1
  - [ ] Clear "document not found" error message
- **Validates**: FR-019
- **Priority**: Medium

#### TS-032: MCP Malformed Request

- **Description**: Verify proper error when MCP server receives a malformed tool call or invalid tool name.
- **Type**: Automated
- **Preconditions**: MCP server running with valid authentication
- **Steps**:
  1. Send an MCP request with an invalid tool name
  2. Send an MCP request with missing required parameters
- **Expected Results**:
  - [ ] Both return appropriate MCP error responses
  - [ ] Server does not crash
- **Validates**: FR-016
- **Priority**: Medium

#### TS-033: Title Below Confidence Threshold

- **Description**: Verify that when AI confidence is below threshold, the system handles the low-confidence title appropriately.
- **Type**: Automated
- **Preconditions**: A test file that is intentionally ambiguous (e.g., abstract art image); confidence threshold set low enough to trigger
- **Steps**:
  1. Index the ambiguous file in non-interactive mode (or with pre-set title override)
  2. Verify the title and confidence are stored
- **Expected Results**:
  - [ ] Document is indexed with the AI-suggested title or a fallback
  - [ ] Confidence score is stored and below threshold
- **Validates**: FR-007
- **Priority**: Medium

#### TS-053: Index File with Special Characters in Path

- **Description**: Verify that files with spaces, accents, and unicode characters in their path are indexed correctly.
- **Type**: Automated
- **Preconditions**: A test file at a path containing spaces and accents (e.g., `tests/fixtures/generated/dossier été/facture café.txt`)
- **Steps**:
  1. Create a directory with accented name and a file with spaces in its name
  2. Run index command on the file with a category
  3. Search for the file content
  4. Run show command using the file path
- **Expected Results**:
  - [ ] File is indexed successfully
  - [ ] File path with special characters is stored correctly
  - [ ] Search returns the file with correct path
  - [ ] Show command works with the special-character path
- **Validates**: FR-001, FR-015
- **Priority**: Medium

#### TS-054: Index Empty File

- **Description**: Verify proper handling of zero-byte or empty-content files.
- **Type**: Automated
- **Preconditions**: A zero-byte .txt file and a PDF with no pages/content
- **Steps**:
  1. Run index command on the empty text file with a valid category
  2. Run index command on the empty PDF with a valid category
- **Expected Results**:
  - [ ] Exit code 1 for each
  - [ ] Clear error message: "file is empty" or "no content to index"
  - [ ] No partial data left in database
- **Validates**: FR-003, FR-002
- **Priority**: Medium

#### TS-055: Index Very Large File

- **Description**: Verify that a large file (>1000 pages PDF or >1MB text) is handled without crashing.
- **Type**: Automated
- **Preconditions**: A large generated text file (~50,000 words)
- **Steps**:
  1. Run index command on the large file with a category
  2. Query database for chunk count
  3. Search for content from the file
- **Expected Results**:
  - [ ] File is indexed successfully (may take longer)
  - [ ] Chunk count is proportional to file size (~500 chunks for 50,000 words at 100w/chunk)
  - [ ] Search returns relevant results from the file
- **Validates**: FR-003, NFR-003
- **Priority**: Low

### 5.6 Untestable Requirements

| Requirement | Reason | Alternative Validation |
|-------------|--------|----------------------|
| FR-007 (confidence accuracy) | AI confidence calibration depends on model behavior and content variability; not deterministically testable | Manual review of confidence scores on a diverse sample of 20+ documents; adjust threshold based on observed accuracy |
| NFR-001, NFR-002 (performance targets) | Performance depends on hardware and API latency; not reproducible in CI | Manual benchmarking on target hardware; track over time |

### 5.7 Test Fixtures

All test fixtures are stored in `tests/fixtures/` and committed to the repository.

#### Provided Fixtures (real files)

These files must be provided by the user or created manually. They serve as templates for tests requiring AI analysis with known expected outputs.

| Fixture File | Type | Used By | Description |
|--------------|------|---------|-------------|
| `official_document.jpg` | Image | TS-001, TS-035 | A scan of an official document (passport, ID, or invoice) with identifiable fields (number, name, dates) |
| `family_photo.jpg` | Image | TS-035 | A photograph with people/scenery (expected: 1 segment "scene_description") |
| `screenshot.png` | Image | TS-034 | A PNG screenshot or diagram |
| `multipage.pdf` | PDF | TS-002, TS-036 | A multi-page PDF (>2 pages) with text and at least 1 embedded image. Page 2 must contain a unique keyword (e.g., "XYZZY") for TS-036 |
| `sample.xlsx` | Spreadsheet | TS-037 | A small XLSX spreadsheet with headers and data |
| `sample.docx` | Document | TS-006 | A DOCX file with text content |
| `ambiguous_image.jpg` | Image | TS-033 | An abstract or ambiguous image likely to produce low AI confidence |

#### Generated Fixtures (created by test setup)

These files are generated programmatically by the test setup script or during test execution.

| Fixture | Type | Used By | Generation |
|---------|------|---------|------------|
| `sample.txt` | Text | TS-004 | Generated: 250 words of lorem ipsum (French) |
| `sample.csv` | Spreadsheet | TS-005 | Generated: 10 rows x 5 columns with headers |
| `sample_en.txt` | Text | TS-020 | Generated: 150 words of English text |
| `sample_fr.txt` | Text | TS-020 | Generated: 150 words of French text |
| `corrupt.jpg` | Image | TS-025 | Generated: zero-byte file with .jpg extension |
| `corrupt.pdf` | PDF | TS-026 | Generated: random bytes with .pdf extension |
| `protected.pdf` | PDF | TS-026 | Generated: password-protected PDF (password: "test") |
| `unsupported.zip` | Binary | TS-022 | Generated: valid ZIP archive |
| `large_text.txt` | Text | TS-003 | Generated: exactly 250 words for chunking accuracy test |
| `unreadable.txt` | Text | TS-051 | Generated: file with no read permission (chmod 000) |
| `corrupt.csv` | Spreadsheet | TS-052 | Generated: random bytes with .csv extension |
| `corrupt.xlsx` | Spreadsheet | TS-052 | Generated: random bytes with .xlsx extension |
| `empty.txt` | Text | TS-054 | Generated: zero-byte .txt file |
| `empty.pdf` | PDF | TS-054 | Generated: valid PDF with no content/pages |
| `large_50k.txt` | Text | TS-055 | Generated: ~50,000 words of lorem ipsum |

#### Fixture Directory Structure

```
tests/fixtures/
├── provided/              # Real files provided by user (gitignored template, see README)
│   ├── official_document.jpg
│   ├── family_photo.jpg
│   ├── screenshot.png
│   ├── multipage.pdf
│   ├── sample.xlsx
│   ├── sample.docx
│   └── ambiguous_image.jpg
├── generated/             # Created by test setup (gitignored, generated on first run)
│   ├── sample.txt
│   ├── sample.csv
│   ├── sample_en.txt
│   ├── sample_fr.txt
│   ├── corrupt.jpg
│   ├── corrupt.pdf
│   ├── corrupt.csv
│   ├── corrupt.xlsx
│   ├── protected.pdf
│   ├── unsupported.zip
│   ├── unreadable.txt
│   ├── empty.txt
│   ├── empty.pdf
│   ├── large_text.txt
│   └── large_50k.txt
├── testdir/               # Directory structure for directory indexing test (TS-039)
│   ├── file1.jpg          # (copy of provided/screenshot.png)
│   ├── file2.txt          # (copy of generated/sample.txt)
│   ├── ignored.zip        # (unsupported file)
│   └── subdir/
│       └── file3.txt      # (copy of generated/sample_en.txt)
├── dossier été/           # Directory with accents for TS-053
│   └── facture café.txt   # File with spaces and accents (copy of generated/sample_fr.txt)
└── README.md              # Instructions for providing fixture files
```

---

## 6. Technical Architecture

### 6.1 Architecture Overview

```
┌──────────────────┐      ┌──────────────────────┐
│    CLI (cobra)   │      │ MCP (Model Context    │
│                  │      │ Protocol) HTTP Server │
│  index           │      │   + OAuth 2.1         │
│  search          │      │                       │
│  categories      │      │   Tools:              │
│  show            │      │   - search            │
│  update          │      │   - index_file        │
│  delete          │      │   - get_document      │
│  status          │      │   - list_categories   │
│  serve ──────────┼──────│   - status            │
└────────┬─────────┘      └───────────┬───────────┘
         │                            │
         └─────────────┬──────────────┘
                       │
               ┌───────▼────────┐
               │   Core Engine  │
               │                │
               │  - Indexer     │
               │  - Searcher    │
               │  - Analyzer    │
               │  - Chunker     │
               └───────┬────────┘
                       │
          ┌────────────┼────────────┐
          │            │            │
  ┌───────▼──────┐ ┌──▼──────────┐ ┌▼─────────────┐
  │  Gemini API  │ │ PostgreSQL  │ │ pdf-extractor │
  │              │ │ + pgvector  │ │   (CLI tool)  │
  │ - Analysis   │ │             │ └───────────────┘
  │   (Flash)    │ │ - documents │
  │ - Embedding  │ │ - chunks    │
  │   (gemini-   │ │ - categories│
  │    embed)    │ │ - images    │
  └──────────────┘ └─────────────┘
```

### 6.2 Technology Stack

| Layer | Technology | Justification |
|-------|------------|---------------|
| Language | Go 1.24+ | Personal preference; fast compilation; single binary deployment; excellent concurrency; skill available |
| CLI framework | cobra | Standard Go CLI framework per golang skill |
| HTTP framework | Fiber | High-performance HTTP framework per golang skill |
| Database | PostgreSQL 16+ with pgvector | Robust relational DB with native vector similarity search (HNSW — Hierarchical Navigable Small World — index) and full-text search (tsvector) |
| ORM | Bun | Lightweight Go ORM per golang skill |
| AI analysis | Gemini 3 Flash Preview (`gemini-3-flash-preview`) via Gemini API | Fast multimodal model for image/document analysis; API-based via GEMINI_API_KEY |
| Embeddings | gemini-embedding-001 via Gemini API | #1 on MTEB (Massive Text Embedding Benchmark) leaderboard (score 68.32); 3072 dims adjustable to 768; $0.15/1M tokens; multilingual |
| PDF extraction | pdf-extractor (local Go binary) | Existing tool for text + image extraction with Gemini analysis; JSON output for programmatic use |
| Document conversion | LibreOffice (headless) | Convert DOC/DOCX/ODT to PDF for processing |
| Logging | slog | Standard library structured logging per golang skill |
| Config | koanf | Configuration management per golang skill |

### 6.3 Key Design Decisions

| Decision | Choice | Alternatives Considered | Rationale | Addresses |
|----------|--------|------------------------|-----------|-----------|
| Embedding model | gemini-embedding-001 (768 dims) | text-embedding-005, OpenAI text-embedding-3, local llama.cpp | #1 MTEB, Google ecosystem alignment, adjustable dimensions (768 for storage efficiency with minimal quality loss: 67.99 vs 68.32), multilingual | FR-009, NFR-015 |
| Vector search | pgvector with HNSW index | Dedicated vector DB (Pinecone, Milvus), FAISS | Single database for everything; HNSW works well for dynamic data without retraining; simplifies deployment | FR-009, NFR-003 |
| Embedding via description | Embed AI-generated text descriptions for images | Multimodal embedding model | gemini-embedding-001 is text-only but #1 ranked; image descriptions from Gemini Flash are rich enough for semantic search | FR-001, FR-009 |
| PDF processing | Shell out to pdf-extractor | Embedded Go library only | Existing reliable tool with Gemini integration; JSON output is easy to parse; avoids reimplementing PDF + AI logic | FR-002 |
| PDF page tracking | Parse page markers in pdf-extractor text output | Add JSON pages array, integrate go-fitz directly | pdf-extractor outputs `--- page N ---` markers at the start of each page and a closing `---` at the end. localfiles-index splits on this pattern to assign page numbers to text chunks. Images already have page_number in the JSON. | FR-002 |
| MCP transport | HTTP Streamable (no SSE) | SSE transport, stdio | User requirement; modern MCP transport; better for network API use case | FR-016 |
| Chunking strategy | 100 words, 5-word overlap | Sentence-based, fixed char count | Word-based chunking is language-agnostic and predictable; overlap ensures context continuity at chunk boundaries | FR-002, FR-003 |
| Auth mechanism | OAuth 2.1 | API key, mTLS | Standard protocol; required for MCP HTTP spec compliance | FR-017, NFR-004 |
| Category storage | PostgreSQL table | Config file, in-memory | Persistent, queryable, supports FK relationships with documents; managed via CLI | FR-013 |

### 6.4 Module Descriptions

#### 6.4.1 CLI Module

##### CLI Reference

###### `localfiles-index index <path>`

Index a file or directory into the search index.

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | Path to file or directory to index |

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--category` | `-c` | (required) | Category name to assign to the document(s). **Required.** |

When a directory is passed as `<path>`, all supported files are indexed recursively (automatic).

**Examples**:
```bash
localfiles-index index ~/Documents/passport.jpg --category administratif
localfiles-index index ~/Documents/work/ --category travail
```

---

###### `localfiles-index search <query>`

Search indexed documents by natural language or keywords.

| Argument | Required | Description |
|----------|----------|-------------|
| `query` | Yes | Free-text search query |

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--mode` | `-m` | `semantic` | Search mode: `semantic` (vector similarity) or `fulltext` (keyword match) |
| `--category` | `-c` | (none) | Filter results to a specific category |
| `--limit` | `-l` | `10` | Maximum number of results to return |
| `--format` | `-f` | `table` | Output format: `table`, `json`, or `detail` |

**Examples**:
```bash
localfiles-index search "passeport Sébastien"
localfiles-index search "facture EDF" --category administratif --limit 5
localfiles-index search "budget 2025" --mode fulltext --format json
```

---

###### `localfiles-index update [path]`

Check indexed documents for changes and re-index modified files. Without a path, checks all indexed documents.

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | No | Path to a specific file to update. If omitted, checks all indexed documents. |

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--force` | `-f` | `false` | Force re-indexing regardless of mtime (bypass change detection) |

**Behavior**:
- Compares current file mtime with stored mtime for each document
- Re-indexes files whose mtime has changed (replaces all chunks and embeddings)
- Skips files whose mtime is unchanged
- Reports files missing from disk as warnings
- Displays a summary: `N updated, N unchanged, N missing`

**Examples**:
```bash
localfiles-index update                          # check all indexed documents
localfiles-index update ~/Documents/passport.jpg # check a specific file
localfiles-index update --force                  # re-index everything
```

---

###### `localfiles-index show <path|id>`

Display full details of an indexed document.

| Argument | Required | Description |
|----------|----------|-------------|
| `path\|id` | Yes | File path or document UUID |

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--chunks` | | `false` | Include chunk content in the output |

**Examples**:
```bash
localfiles-index show ~/Documents/passport.jpg
localfiles-index show ~/Documents/passport.jpg --chunks
localfiles-index show 550e8400-e29b-41d4-a716-446655440000
```

---

###### `localfiles-index delete <path|id>`

Remove a document and all associated data (chunks, embeddings, images) from the index.

| Argument | Required | Description |
|----------|----------|-------------|
| `path\|id` | Yes | File path or document UUID |

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--yes` | `-y` | `false` | Skip confirmation prompt |

**Examples**:
```bash
localfiles-index delete ~/Documents/old-file.pdf
localfiles-index delete 550e8400-e29b-41d4-a716-446655440000 --yes
```

---

###### `localfiles-index categories <action>`

Manage document categories.

| Action | Arguments | Description |
|--------|-----------|-------------|
| `list` | (none) | List all categories |
| `add <name>` | `name` (required) | Create a new category |
| `remove <name>` | `name` (required) | Delete a category |
| `update <name>` | `name` (required) | Update a category |

| Flag | Applies to | Default | Description |
|------|------------|---------|-------------|
| `--description` | `add`, `update` | (none) | Category description |
| `--force` | `remove` | `false` | Delete even if documents reference this category (sets their category to null) |

**Examples**:
```bash
localfiles-index categories list
localfiles-index categories add administratif --description "Documents administratifs"
localfiles-index categories update administratif --description "Documents admin et juridiques"
localfiles-index categories remove old-category --force
```

---

###### `localfiles-index status`

Display index statistics.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | `-f` | `table` | Output format: `table` or `json` |

**Examples**:
```bash
localfiles-index status
localfiles-index status --format json
```

---

###### `localfiles-index serve`

Start the MCP HTTP Streamable server.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `8080` | Port to listen on (also configurable via `MCP_PORT` env var) |
| `--credentials` | | `~/.credentials/scm-pwd-web.json` | Path to OAuth 2.1 credentials file |

**Examples**:
```bash
localfiles-index serve
localfiles-index serve --port 9090 --credentials /path/to/creds.json
```

---

###### Global Flags

These flags apply to all subcommands:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--verbose` | `-v` | `false` | Enable debug-level logging |
| `--help` | `-h` | | Show help for any command |

##### User Flows

**Flow: Index a File**

1. User runs `localfiles-index index ~/Documents/passport.jpg --category administratif`
2. System detects file type (image) from extension and MIME type
3. System records file mtime and checks for existing record
4. If new: System sends image to Gemini Flash for analysis → receives description, type, caption
5. System generates title with confidence score
6. If confidence < threshold: System prompts user "Suggested title: 'Passeport Sébastien Morand' (confidence: 0.85). Accept? [Y/n/edit]:"
7. System creates embedding from the description text
8. System stores document, chunk (with embedding), and image records
9. System displays confirmation with document ID and title

**Flow: Search**

1. User runs `localfiles-index search "passeport Sébastien"`
2. System generates embedding for the query (using RETRIEVAL_QUERY task type)
3. System queries pgvector for top-N most similar chunks
4. System joins with documents and categories for full result context
5. System formats and displays results in the requested format

> **Data models**: See [6.4.4 Data Storage](#644-data-storage)

---

#### 6.4.2 MCP HTTP Server Module

##### API Overview

- **Transport**: MCP HTTP Streamable (NOT SSE)
- **Authentication**: OAuth 2.1
- **Default Port**: 8080 (configurable via `MCP_PORT` env var)

##### MCP Tools

| Tool Name | Description | Parameters |
|-----------|-------------|------------|
| `search` | Search indexed documents | `query` (string, required), `mode` (string: "semantic"\|"fulltext", default: "semantic"), `category` (string, optional), `limit` (int, default: 10) |
| `index_file` | Index a file by path | `path` (string, required), `category` (string, required) |
| `get_document` | Get document details | `id` (string, optional), `path` (string, optional) — one required |
| `list_categories` | List all categories | (none) |
| `delete_document` | Delete a document | `id` (string, optional), `path` (string, optional) — one required |
| `status` | Index statistics | (none) |
| `update` | Re-index modified documents | `path` (string, optional — omit for all), `force` (bool, default: false) |

##### Tool Response Format

All MCP tool responses follow the standard MCP tool result format with `content` blocks containing either `text` (JSON string) or `image` (base64) types.

> **Data models**: See [6.4.4 Data Storage](#644-data-storage)

---

#### 6.4.3 Core Engine

##### Components

| Component | Purpose | Key Functions |
|-----------|---------|---------------|
| Indexer | Orchestrates file indexing pipeline | Detect file type → extract content → analyze → chunk → embed → store |
| Searcher | Executes search queries | Embed query → vector/fulltext search → rank → format results |
| Analyzer | AI-based content analysis | Call Gemini Flash for descriptions, titles, confidence scores |
| Chunker | Text splitting | Split text into fixed-word chunks with configurable overlap |
| Embedder | Embedding generation | Call gemini-embedding-001 with appropriate task types |

##### Indexing Pipeline

```
File Input
    │
    ▼
┌───────────────────┐
│ Detect File Type   │
│ (extension + MIME) │
└─────────┬─────────┘
          │
    ┌─────┴─────┬────────────┬──────────────┐
    ▼           ▼            ▼              ▼
 Image        PDF        Text File     Spreadsheet
    │           │            │              │
    ▼           ▼            ▼              ▼
 Gemini     pdf-extractor  Read         Read content
 Flash      (shell out)    content      → Gemini Flash
 analyze       │            │           describe
    │     ┌────┴────┐       │              │
    │     ▼         ▼       │              │
    │   Text     Images     │              │
    │  (+ page   (+ page    │              │
    │   numbers)  numbers)  │              │
    │     │         │       │              │
    │     ▼         ▼       ▼              ▼
    └─────┴────┬────┴───────┴──────────────┘
               │
               ▼
    ┌─────────────────────────┐
    │  Gemini Flash generates │
    │  for ALL document types:│
    │  ─ Title + confidence   │
    │  ─ Summary (~100 words) │
    └────────────┬────────────┘
                 │
                 ▼
    ┌─────────────────────────┐
    │  Create segments:       │
    │                         │
    │  1. doc_title chunk     │
    │  2. doc_summary chunk   │
    │  3. text chunks         │
    │     (100w/5w overlap)   │
    │     + source_page       │
    │  4. Per image:          │
    │     image_summary chunk │
    │     image_description   │
    │     chunk + source_page │
    └────────────┬────────────┘
                 │
                 ▼
        Generate Embeddings
        (gemini-embedding-001)
        task_type: RETRIEVAL_DOCUMENT
        → 1 embedding per segment
                 │
                 ▼
        Store in PostgreSQL
        (document + chunks + images)
```

##### Segment Types per Document Type

| Document Type | doc_title | doc_summary | text chunks | image_segment (AI-driven) |
|---------------|-----------|-------------|-------------|---------------------------|
| Image | — | — | — | K (AI decides: 1 for a photo, 3–5 for an official document) |
| PDF | 1 | 1 | N (100w/5w) | K per extracted image |
| Text file | 1 | 1 | N (100w/5w) | — |
| Spreadsheet | 1 | 1 | 1 (full desc) | — |

**Examples**:
- A passport image = 4 image_segments (passport_number, holder_name, description, dates) = **4 embeddings**
- A family photo = 1 image_segment (scene_description) = **1 embedding**
- A 500-word PDF with 2 images (1 chart + 1 diagram) = 1 (title) + 1 (summary) + 5 (text) + 2 (chart) + 3 (diagram) = **~12 embeddings**

##### Other Document Formats (DOC, DOCX, ODT, Google Docs exports, ...)

```
File Input (.docx, .doc, .odt, .gdoc, ...)
    │
    ▼
┌─────────────────────────────┐
│ Convert to temporary PDF    │
│ (LibreOffice headless)      │
│                             │
│ NOTE: original file_path    │
│ is preserved in the         │
│ document record — the PDF   │
│ is only a processing step   │
└──────────────┬──────────────┘
               │
               ▼
    Process as PDF (same pipeline as above)
               │
               ▼
    Clean up temporary PDF
```

##### Interactions

| Interacts With | Direction | Protocol | Purpose |
|----------------|-----------|----------|---------|
| Gemini API (Flash) | Outbound | HTTPS REST | Image/document analysis, title generation |
| Gemini API (Embedding) | Outbound | HTTPS REST | Generate embeddings for chunks and queries |
| PostgreSQL | Outbound | TCP (pg protocol) | Store and query documents, chunks, categories |
| pdf-extractor | Outbound | Process exec (stdout JSON) | Extract text and images from PDFs; text output contains `--- page N ---` markers at the start of each page and `---` at the end; images include page_number in JSON |
| LibreOffice | Outbound | Process exec | Convert document formats to PDF |

> **Data models**: See [6.4.4 Data Storage](#644-data-storage)

---

#### 6.4.4 Data Storage

##### Data Models

**categories**

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, NOT NULL, DEFAULT gen_random_uuid() | Unique identifier |
| name | TEXT | UNIQUE, NOT NULL | Category name (e.g., "administratif") |
| description | TEXT | | Optional description |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |

**documents**

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, NOT NULL, DEFAULT gen_random_uuid() | Unique identifier |
| file_path | TEXT | UNIQUE, NOT NULL | Absolute path to the original file |
| file_mtime | TIMESTAMP | NOT NULL | File modification time (mtime) at last indexing, for change detection |
| title | TEXT | NOT NULL | AI-generated or user-provided document title |
| title_confidence | REAL | | Confidence score (0.0–1.0) of the AI-generated title |
| document_type | TEXT | NOT NULL | File type: "image", "pdf", "text", "spreadsheet", "other" |
| category_id | UUID | FK → categories(id) ON DELETE SET NULL | Assigned category |
| mime_type | TEXT | | MIME type of the file |
| file_size | BIGINT | | File size in bytes |
| metadata | JSONB | DEFAULT '{}' | Additional metadata (flexible key-value) |
| indexed_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | When the file was last indexed |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Record creation time |
| updated_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Record last update time |

**chunks**

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, NOT NULL, DEFAULT gen_random_uuid() | Unique identifier |
| document_id | UUID | FK → documents(id) ON DELETE CASCADE, NOT NULL | Parent document |
| chunk_index | INTEGER | NOT NULL | Position in the document (0-based) |
| content | TEXT | NOT NULL | Chunk text content |
| chunk_type | TEXT | NOT NULL | Type: "doc_title", "doc_summary", "text", "image_segment" |
| chunk_label | TEXT | | AI-assigned label for the segment (e.g., "passport_number", "holder_name", "scene_description") — populated for image_segment chunks |
| source_page | INTEGER | | Page number in source document (null for standalone images, document-level chunks) |
| embedding | vector(768) | | Embedding vector (dimension configurable) |
| search_vector | tsvector | | Full-text search index (auto-generated from content) |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |

**images**

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, NOT NULL, DEFAULT gen_random_uuid() | Unique identifier |
| document_id | UUID | FK → documents(id) ON DELETE CASCADE, NOT NULL | Parent document |
| chunk_id | UUID | FK → chunks(id) ON DELETE SET NULL | Associated primary chunk — for standalone images, this links to the first (most descriptive) image_segment chunk; for PDF-extracted images, this links to the image_description chunk |
| image_path | TEXT | NOT NULL | Path to the extracted image file |
| description | TEXT | | AI-generated image description |
| image_type | TEXT | | Type: "photograph", "diagram", "chart", "illustration", etc. |
| caption | TEXT | | AI-generated caption |
| source_page | INTEGER | | Page number in source PDF (null for standalone images) |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |

##### Relationships

```
categories 1──────────0..N documents
documents  1──────────0..N chunks
documents  1──────────0..N images
chunks     1──────────0..1 images (primary chunk links to image; other image_segment chunks reference same document but not the image record directly)
```

##### Indexes

| Index | Type | Column(s) | Purpose |
|-------|------|-----------|---------|
| idx_chunks_embedding | HNSW (vector_cosine_ops) | chunks.embedding | Fast vector similarity search |
| idx_chunks_search_vector | GIN | chunks.search_vector | Fast full-text search |
| idx_documents_category | B-tree | documents.category_id | Category filter queries |
| idx_documents_file_path | B-tree | documents.file_path | Unique file path lookups |
| idx_chunks_document | B-tree | chunks.document_id | Chunk-to-document joins |

##### Auto-generated search_vector

A database trigger automatically populates the `search_vector` column when a chunk is inserted or updated:

```sql
CREATE FUNCTION update_search_vector() RETURNS trigger AS $$
BEGIN
  NEW.search_vector := to_tsvector('simple', NEW.content);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

Using the `'simple'` text search configuration for multilingual support (no language-specific stemming — handles French and English content equally).

##### Configuration

```yaml
# Database configuration
database:
  url: "${DATABASE_URL}"   # NFR-006 (Data Integrity) — ACID transactions
  max_connections: 10      # NFR-003 (Index Capacity)

# Vector index
vector:
  dimensions: 768          # NFR-001 (Performance) — 768 balances quality vs speed
  index_type: hnsw         # NFR-003 (Scalability) — no retraining needed
```

##### Migration Strategy

- **Approach**: Auto-migration on first startup using Bun's migration system
- **Schema versioning**: Sequential numbered migrations in `internal/storage/migrations/`
- **Rollback**: Each migration includes an `Up()` and `Down()` function
- **pgvector extension**: First migration creates `CREATE EXTENSION IF NOT EXISTS vector`

### 6.5 Identity & Permissions

#### Authentication

- **Method**: OAuth 2.1 (for MCP HTTP server only; CLI operates locally without auth)
- **Flow**: Client Credentials Grant (machine-to-machine)
- **Credential source**: JSON file at path specified by `OAUTH_CREDENTIALS_PATH` env var
- **Credential format**: Google OAuth JSON format (example: `~/.credentials/scm-pwd-web.json`)
  ```json
  {
    "web": {
      "client_id": "...",
      "client_secret": "...",
      "token_uri": "...",
      ...
    }
  }
  ```
- **Token lifecycle**: Access tokens with configurable expiry (default: 1 hour)

#### Test Authentication

- **Test credentials**: Dedicated test client_id/client_secret in a test-only credentials file
- **Test tokens**: Generated during test setup, valid for test duration
- **Environment isolation**: Test credentials file separate from production credentials

#### Permission Model

| Role | Description | Scope |
|------|-------------|-------|
| authenticated_client | Any client with valid OAuth token | Full access to all MCP tools |
| unauthenticated | No valid token | No access — all requests rejected |

#### Permission Matrix

| Action | authenticated_client | unauthenticated |
|--------|---------------------|-----------------|
| Search | ✅ | ❌ |
| Index file | ✅ | ❌ |
| Get document | ✅ | ❌ |
| List categories | ✅ | ❌ |
| Delete document | ✅ | ❌ |
| Update | ✅ | ❌ |
| Status | ✅ | ❌ |

### 6.6 Dependencies

#### External Services

| Service | Purpose | Criticality | Fallback |
|---------|---------|-------------|----------|
| Gemini API (gemini-3-flash-preview) | Content analysis, title generation, image description | Critical | Indexing fails gracefully with error message; skip file |
| Gemini API (gemini-embedding-001) | Text embedding generation | Critical | Indexing fails gracefully; file skipped |

#### Internal Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| pdf-extractor (local binary) | Available at `~/.local/bin/pdf-extractor` | Used for PDF text/image extraction; path configurable via `PDF_EXTRACTOR_PATH`. Text output includes `--- page N ---` markers at the start of each page and a closing `---` at the end. |
| LibreOffice (headless) | Must be installed for DOC/DOCX conversion | Optional: only needed for non-PDF document formats |
| PostgreSQL 16+ with pgvector | Must be running locally | Connection: `postgresql://localfiles:localfiles@localhost:5432/localfiles` |

#### Third-Party Libraries

| Library | Version | License | Purpose |
|---------|---------|---------|---------|
| github.com/spf13/cobra | latest | Apache 2.0 | CLI framework |
| github.com/gofiber/fiber/v2 | latest | MIT | HTTP framework for MCP server |
| github.com/uptrace/bun | latest | BSD-2 | PostgreSQL ORM |
| github.com/pgvector/pgvector-go | latest | MIT | pgvector Go client |
| github.com/knadh/koanf | latest | MIT | Configuration management |
| google.golang.org/genai | latest | Apache 2.0 | Google Generative AI SDK (Gemini API) |

### 6.7 Observability

#### Log Standards

- **Format**: Structured JSON via Go `slog`
- **Fields**: timestamp, level, module, message, metadata (key-value pairs)
- **Levels**: DEBUG (SQL, API calls), INFO (operations), WARN (degraded state), ERROR (failures)

```json
{
  "time": "2026-02-10T10:30:00.123Z",
  "level": "INFO",
  "msg": "file indexed successfully",
  "module": "indexer",
  "file_path": "/Users/sebastien/Documents/passport.jpg",
  "document_id": "uuid",
  "chunks": 1,
  "duration_ms": 2340
}
```

#### Log Organization

| Module | Output | Level Control |
|--------|--------|---------------|
| All modules | stderr (default) or configurable output | `LOG_LEVEL` env var (default: info) |

Single-binary application — all logs go to stderr with module field for filtering. No file-based log rotation needed for personal tool.

#### Metrics & Dashboards

Not required for v1 (personal tool). Logs provide sufficient observability.

#### Tracing

Not required for v1 (personal tool, local only).

### 6.8 DevOps

#### Version Control

- **Repository**: `localfiles-index` on GitHub (personal account: smorand)
- **Hosting**: GitHub
- **Branching strategy**: Single branch (`main`) — personal project
- **Push policy**: Push after each significant implementation
- **Protected branches**: None (personal repo)

#### Build & Automation

| Target / Command | Description | When to Run |
|------------------|-------------|-------------|
| `make build` | Compile binary to `bin/localfiles-index-{os}-{arch}` | Before test/run |
| `make run CMD=<subcommand>` | Build and run with subcommand | During development |
| `make test-unit` | Run Go unit tests | During development |
| `make test` | Run functional tests (shell scripts in `tests/`) | Before push |
| `make fmt` | Format code with gofmt | Before commit |
| `make vet` | Run go vet analysis | Before commit |
| `make check` | All checks (fmt + vet + test) | Before push |
| `make install` | Install binary to `~/.local/bin/` | After build |
| `make clean` | Remove build artifacts | As needed |
| `make db-setup` | Create PostgreSQL database and run migrations | Initial setup |

#### CI/CD Pipeline

- **CI Tool**: None (local only for v1)
- **Local workflow**: `make check` before push

#### Environments

| Environment | Purpose | Deploy Method | URL |
|-------------|---------|---------------|-----|
| Local | Development + Production | `make install` | localhost:8080 (MCP) |

### 6.9 Code Structure

#### Project Layout

```
localfiles-index/
├── Makefile                      # Build automation (6.8)
├── CLAUDE.md                     # AI-oriented doc (compact index)
├── README.md                     # Human-oriented documentation
├── specifications.md             # This file
├── .agent_docs/                  # Detailed documentation
│   ├── golang.md                 # Go coding standards
│   └── makefile.md               # Makefile documentation
├── .gitignore                    # Git ignore patterns
├── go.mod                        # Go module definition
├── go.sum                        # Dependency checksums
├── cmd/                          # Main application entry point
│   └── localfiles-index/
│       └── main.go               # Entry point (minimal — wiring only)
├── internal/                     # Private application code
│   ├── config/                   # Configuration (env vars, defaults)
│   │   └── config.go
│   ├── storage/                  # PostgreSQL operations
│   │   ├── storage.go            # Repository interface + implementation
│   │   ├── models.go             # Data models (Go structs)
│   │   └── migrations/           # Database migrations
│   │       └── 001_initial.go
│   ├── indexer/                  # File indexing pipeline
│   │   ├── indexer.go            # Orchestrator
│   │   ├── chunker.go            # Text chunking logic
│   │   └── detector.go           # File type detection
│   ├── analyzer/                 # AI-based content analysis
│   │   └── analyzer.go           # Gemini Flash integration
│   ├── embedding/                # Embedding generation
│   │   └── embedding.go          # Gemini embedding API client
│   ├── searcher/                 # Search logic
│   │   └── searcher.go           # Semantic + full-text search
│   ├── mcp/                      # MCP HTTP server
│   │   ├── server.go             # HTTP Streamable server setup
│   │   ├── tools.go              # MCP tool definitions
│   │   └── oauth.go              # OAuth 2.1 middleware
│   └── cli/                      # CLI commands (cobra)
│       ├── root.go               # Root command + global flags
│       ├── index.go              # index subcommand
│       ├── search.go             # search subcommand
│       ├── categories.go         # categories subcommand
│       ├── show.go               # show subcommand
│       ├── update.go             # update subcommand
│       ├── delete.go             # delete subcommand
│       ├── status.go             # status subcommand
│       └── serve.go              # serve subcommand (MCP server)
└── tests/                        # Functional tests (shell scripts)
    ├── run_tests.sh              # Test runner
    ├── test_index.sh             # Indexing integration tests
    ├── test_search.sh            # Search integration tests
    ├── test_categories.sh        # Category CRUD tests
    ├── test_cli_workflow.sh       # Full CLI E2E workflow
    ├── test_mcp.sh               # MCP HTTP E2E tests
    └── fixtures/                 # Test data (see section 5.7)
        ├── provided/             # Real files provided by user (gitignored)
        │   ├── official_document.jpg
        │   ├── family_photo.jpg
        │   ├── screenshot.png
        │   ├── multipage.pdf
        │   ├── sample.xlsx
        │   ├── sample.docx
        │   └── ambiguous_image.jpg
        ├── generated/            # Created by test setup (gitignored)
        │   ├── sample.txt
        │   ├── sample.csv
        │   └── ...
        ├── testdir/              # Directory structure for directory indexing test
        └── README.md             # Fixture setup instructions
```

#### Key Conventions

| Convention | Standard | Source |
|------------|----------|--------|
| Naming style | camelCase (unexported), PascalCase (exported) | Go convention |
| Package structure | By domain/feature (not by layer) | golang skill |
| Config format | Environment variables (koanf) | golang skill |
| Test location | Unit tests co-located (`_test.go`), functional tests in `tests/` | golang skill |
| Error handling | Wrapped errors with `%w`, sentinel errors for expected conditions | golang skill |
| Logging | `slog` structured logging | golang skill |
| Binary output | `bin/` directory | golang skill Makefile |

#### Skill Alignment

- **Primary skill**: golang
- **Conventions inherited**: cmd/ structure, internal/ organization, Makefile with standard targets, cobra CLI, slog logging, Bun ORM, Fiber HTTP, functional tests in `tests/`
- **Deviations**: None

#### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Yes | `postgresql://localfiles:localfiles@localhost:5432/localfiles` |
| `GEMINI_API_KEY` | Google Gemini API key | Yes | — |
| `GEMINI_MODEL` | Model for content analysis | No | `gemini-3-flash-preview` |
| `EMBEDDING_MODEL` | Model for embeddings | No | `gemini-embedding-001` |
| `EMBEDDING_DIMENSIONS` | Embedding vector dimensions | No | `768` |
| `OAUTH_CREDENTIALS_PATH` | Path to OAuth 2.1 credentials JSON | For MCP server | — |
| `MCP_PORT` | MCP HTTP server port | No | `8080` |
| `TITLE_CONFIDENCE_THRESHOLD` | Minimum confidence for auto-accepting AI title (0.0–1.0) | No | `0.9` |
| `CHUNK_SIZE` | Number of words per chunk | No | `100` |
| `CHUNK_OVERLAP` | Overlap words between consecutive chunks | No | `5` |
| `PDF_EXTRACTOR_PATH` | Path to pdf-extractor binary | No | `pdf-extractor` (from PATH) |
| `LOG_LEVEL` | Log level: debug, info, warn, error | No | `info` |

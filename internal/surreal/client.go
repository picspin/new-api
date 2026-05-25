package surreal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

type Client struct {
	db *surrealdb.DB
}

func NewClient(url, user, pass, namespace, database string) (*Client, error) {
	db, err := surrealdb.New(url)
	if err != nil {
		return nil, fmt.Errorf("surreal connect failed: %w", err)
	}
	if err := db.Use(context.Background(), namespace, database); err != nil {
		return nil, fmt.Errorf("surreal use namespace/db failed: %w", err)
	}
	if _, err := db.SignIn(context.Background(), map[string]any{"user": user, "pass": pass}); err != nil {
		return nil, fmt.Errorf("surreal signin failed: %w", err)
	}
	return &Client{db: db}, nil
}

// Notebook record as stored in SurrealDB.
type Notebook struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// Workspace record.
type Workspace struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title,omitempty"`
	Lang  string `json:"lang,omitempty"`
	Owner string `json:"owner,omitempty"`
}

// Source record.
type Source struct {
	ID            string `json:"id"`
	Workspace     string `json:"workspace"`
	DocumentTitle string `json:"document_title,omitempty"`
	FileRef       string `json:"file_ref,omitempty"`
	FileName      string `json:"file_name,omitempty"`
	FileSizeBytes int64  `json:"file_size_bytes,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
	ChunkCount    int    `json:"chunk_count,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

// Chunk record.
type Chunk struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Text     string `json:"text,omitempty"`
	PageRef  string `json:"page_ref,omitempty"`
	Position int    `json:"position,omitempty"`
	Citation string `json:"citation_ref,omitempty"`
}

func (c *Client) GetNotebook(ctx context.Context, notebookID string) (*Notebook, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	results, err := surrealdb.Query[[]Notebook](ctx, c.db, "SELECT * FROM notebooks WHERE id = $id", map[string]any{"id": notebookID})
	if err != nil {
		return nil, fmt.Errorf("surreal get notebook failed: %w", err)
	}
	if len(*results) == 0 {
		return nil, nil
	}
	r := (*results)[0]
	if r.Error != nil {
		return nil, fmt.Errorf("surreal get notebook query error: %w", r.Error)
	}
	if len(r.Result) == 0 {
		return nil, nil
	}
	return &r.Result[0], nil
}

func (c *Client) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	results, err := surrealdb.Query[[]Workspace](ctx, c.db, "SELECT * FROM workspaces WHERE id = $id", map[string]any{"id": workspaceID})
	if err != nil {
		return nil, fmt.Errorf("surreal get workspace failed: %w", err)
	}
	if len(*results) == 0 {
		return nil, nil
	}
	r := (*results)[0]
	if r.Error != nil {
		return nil, fmt.Errorf("surreal get workspace query error: %w", r.Error)
	}
	if len(r.Result) == 0 {
		return nil, nil
	}
	return &r.Result[0], nil
}

func (c *Client) GetSourcesByWorkspace(ctx context.Context, workspaceID string) ([]Source, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	results, err := surrealdb.Query[[]Source](ctx, c.db, "SELECT * FROM sources WHERE workspace = $workspace", map[string]any{"workspace": workspaceID})
	if err != nil {
		return nil, fmt.Errorf("surreal get sources failed: %w", err)
	}
	if len(*results) == 0 {
		return nil, nil
	}
	r := (*results)[0]
	if r.Error != nil {
		return nil, fmt.Errorf("surreal get sources query error: %w", r.Error)
	}
	return r.Result, nil
}

func (c *Client) GetSourceByID(ctx context.Context, sourceID string) (*Source, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	results, err := surrealdb.Query[[]Source](ctx, c.db, "SELECT * FROM sources WHERE id = $id", map[string]any{"id": sourceID})
	if err != nil {
		return nil, fmt.Errorf("surreal get source failed: %w", err)
	}
	if len(*results) == 0 {
		return nil, nil
	}
	r := (*results)[0]
	if r.Error != nil {
		return nil, fmt.Errorf("surreal get source query error: %w", r.Error)
	}
	if len(r.Result) == 0 {
		return nil, nil
	}
	return &r.Result[0], nil
}

func (c *Client) GetChunksBySources(ctx context.Context, sourceIDs []string) ([]Chunk, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if len(sourceIDs) == 0 {
		return nil, nil
	}

	results, err := surrealdb.Query[[]Chunk](ctx, c.db, "SELECT * FROM chunks WHERE source IN $sources ORDER BY position", map[string]any{"sources": sourceIDs})
	if err != nil {
		return nil, fmt.Errorf("surreal get chunks failed: %w", err)
	}
	if len(*results) == 0 {
		return nil, nil
	}
	r := (*results)[0]
	if r.Error != nil {
		return nil, fmt.Errorf("surreal get chunks query error: %w", r.Error)
	}
	return r.Result, nil
}

func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close(context.Background())
	}
	return nil
}

var ErrWorkspaceUnavailable = errors.New("workspace_unavailable")

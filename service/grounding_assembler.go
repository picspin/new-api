package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/internal/surreal"
)

var (
	ErrNotebookNotFound      = errors.New("notebook_not_found")
	ErrSourcesNotFound       = errors.New("sources_not_found")
	ErrWorkspaceUnavailable  = errors.New("workspace_unavailable")
)

type GroundingAssembler struct {
	surrealClient *surreal.Client
}

func NewGroundingAssembler(surrealClient *surreal.Client) *GroundingAssembler {
	return &GroundingAssembler{surrealClient: surrealClient}
}

// GroundingBundle is the canonical output of Lane D.
type GroundingBundle struct {
	BundleID       string          `json:"bundle_id"`
	NotebookID     string          `json:"notebook_id"`
	WorkspaceID    string          `json:"workspace_id"`
	WorkspaceTitle string          `json:"workspace_title"`
	Language       string          `json:"language"`
	Sources        []SourceContent `json:"sources"`
	AssembledAt    time.Time       `json:"assembled_at"`
}

// SourceContent is a source with its chunks.
type SourceContent struct {
	SourceID       string   `json:"source_id"`
	DocumentTitle  string   `json:"document_title"`
	FileRef        string   `json:"file_ref"`
	ChunkCount     int      `json:"chunk_count"`
	Citations      []string `json:"citation_refs"`
	Chunks         []Chunk  `json:"chunks"`
}

type Chunk struct {
	ChunkID     string `json:"chunk_id"`
	PageRef     string `json:"page_ref"`
	Text        string `json:"text"`
	CitationRef string `json:"citation_ref"`
}

type AssembleInput struct {
	NotebookID  string
	SourceRefs  []string // optional filter; nil means all sources
}

func (a *GroundingAssembler) AssembleGroundingBundle(ctx context.Context, input AssembleInput) (*GroundingBundle, error) {
	// 1. Get notebook
	notebook, err := a.surrealClient.GetNotebook(ctx, input.NotebookID)
	if err != nil {
		return nil, fmt.Errorf("notebook lookup failed: %w", err)
	}
	if notebook == nil {
		return nil, ErrNotebookNotFound
	}

	// 2. Get workspace for title/language
	workspace, err := a.surrealClient.GetWorkspace(ctx, notebook.Workspace)
	if err != nil {
		return nil, fmt.Errorf("workspace lookup failed: %w", err)
	}

	// 3. Get all sources in workspace
	allSources, err := a.surrealClient.GetSourcesByWorkspace(ctx, notebook.Workspace)
	if err != nil {
		return nil, fmt.Errorf("sources lookup failed: %w", err)
	}

	if len(allSources) == 0 {
		return &GroundingBundle{
			BundleID:       generateBundleID(),
			NotebookID:     input.NotebookID,
			WorkspaceID:    notebook.Workspace,
			WorkspaceTitle: workspaceTitle(workspace, notebook),
			Language:       detectLanguage(workspace, notebook),
			Sources:        []SourceContent{},
			AssembledAt:    time.Now().UTC(),
		}, nil
	}

	// 4. Filter sources if source_refs provided
	sources := filterSources(allSources, input.SourceRefs)
	if len(sources) == 0 && len(input.SourceRefs) > 0 {
		return nil, ErrSourcesNotFound
	}

	// 5. Get chunks for all relevant sources
	sourceIDs := make([]string, len(sources))
	for i, s := range sources {
		sourceIDs[i] = s.ID
	}

	chunks, err := a.surrealClient.GetChunksBySources(ctx, sourceIDs)
	if err != nil {
		return nil, fmt.Errorf("chunks lookup failed: %w", err)
	}

	// 6. Group chunks by source
	chunksBySource := groupChunksBySource(chunks)

	// 7. Assemble source content
	sourceContents := make([]SourceContent, 0, len(sources))
	citationSet := make(map[string]struct{})
	for _, src := range sources {
		srcChunks := chunksBySource[src.ID]
		sort.Slice(srcChunks, func(i, j int) bool { return srcChunks[i].Position < srcChunks[j].Position })

		chunks := make([]Chunk, len(srcChunks))
		for i, c := range srcChunks {
			chunks[i] = Chunk{
				ChunkID:     c.ID,
				PageRef:     c.PageRef,
				Text:        c.Text,
				CitationRef: c.Citation,
			}
			if c.Citation != "" {
				citationSet[c.Citation] = struct{}{}
			}
		}

		citations := make([]string, 0, len(citationSet))
		for k := range citationSet {
			citations = append(citations, k)
		}

		sourceContents = append(sourceContents, SourceContent{
			SourceID:       src.ID,
			DocumentTitle:  src.DocumentTitle,
			FileRef:        src.FileRef,
			ChunkCount:     len(srcChunks),
			Chunks:         chunks,
			Citations:      citations,
		})
	}

	return &GroundingBundle{
		BundleID:       generateBundleID(),
		NotebookID:     input.NotebookID,
		WorkspaceID:    notebook.Workspace,
		WorkspaceTitle: workspaceTitle(workspace, notebook),
		Language:       detectLanguage(workspace, notebook),
		Sources:        sourceContents,
		AssembledAt:    time.Now().UTC(),
	}, nil
}

func filterSources(all []surreal.Source, ids []string) []surreal.Source {
	if len(ids) == 0 {
		return all
	}
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	var out []surreal.Source
	for _, s := range all {
		if _, ok := idSet[s.ID]; ok {
			out = append(out, s)
		}
	}
	return out
}

func groupChunksBySource(chunks []surreal.Chunk) map[string][]surreal.Chunk {
	m := make(map[string][]surreal.Chunk)
	for _, c := range chunks {
		m[c.Source] = append(m[c.Source], c)
	}
	return m
}

func workspaceTitle(ws *surreal.Workspace, nb *surreal.Notebook) string {
	if ws != nil && ws.Title != "" {
		return ws.Title
	}
	if nb != nil && nb.Name != "" {
		return nb.Name
	}
	return "Untitled"
}

func detectLanguage(ws *surreal.Workspace, nb *surreal.Notebook) string {
	if ws != nil && ws.Lang != "" {
		return ws.Lang
	}
	if nb != nil && nb.Language != "" {
		return nb.Language
	}
	return "en"
}

func generateBundleID() string {
	return "gb_" + time.Now().Format("20060102150405")
}

// buildGroundingContent serializes a grounding bundle into a text string
// suitable for injection into the open-notebook Content field.
func BuildGroundingContent(bundle *GroundingBundle) string {
	if bundle == nil {
		return ""
	}
	var b string
	b += "Workspace: " + bundle.WorkspaceTitle + "\n"
	b += "Language: " + bundle.Language + "\n"
	b += "Sources:\n"
	for _, src := range bundle.Sources {
		b += fmt.Sprintf("\n## %s\n", src.DocumentTitle)
		for _, chunk := range src.Chunks {
			page := ""
			if chunk.PageRef != "" {
				page = " (p." + chunk.PageRef + ")"
			}
			b += fmt.Sprintf("[%s%s] %s\n", chunk.CitationRef, page, chunk.Text)
		}
	}
	return b
}

func buildGroundingContent(bundle *GroundingBundle) string {
	return BuildGroundingContent(bundle)
}

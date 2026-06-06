// Package rag provides Retrieval-Augmented Generation pipeline tools for agent-go.
//
// The pack composes chunking, embedding, vector storage, and reranking into a
// cohesive RAG pipeline. All heavy operations are delegated to pluggable interfaces,
// keeping the pack free of external dependencies beyond agent-go core.
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	// Embed generates an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float64, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// VectorStore provides persistent vector storage and retrieval.
type VectorStore interface {
	// Upsert stores or updates a document chunk with its vector and metadata.
	Upsert(ctx context.Context, indexName string, doc Document) error

	// UpsertBatch stores or updates multiple documents.
	UpsertBatch(ctx context.Context, indexName string, docs []Document) error

	// Search finds the nearest documents to a query vector.
	Search(ctx context.Context, indexName string, query []float64, limit int, filter map[string]any) ([]ScoredDocument, error)

	// Delete removes a document by ID from an index.
	Delete(ctx context.Context, indexName string, id string) error

	// DeleteIndex removes an entire index and all its documents.
	DeleteIndex(ctx context.Context, indexName string) error
}

// Reranker reorders search results by relevance to the original query text.
type Reranker interface {
	// Rerank scores and reorders documents against the query.
	Rerank(ctx context.Context, query string, docs []ScoredDocument, topN int) ([]ScoredDocument, error)
}

// Chunker splits text into smaller segments suitable for embedding.
type Chunker interface {
	// Chunk splits text into chunks.
	Chunk(ctx context.Context, text string, opts ChunkOptions) ([]Chunk, error)
}

// Document represents a stored document chunk with its embedding.
type Document struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Vector   []float64      `json:"vector,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ScoredDocument is a document with a similarity score.
type ScoredDocument struct {
	Document
	Score float64 `json:"score"`
}

// Chunk represents a segment of text produced by chunking.
type Chunk struct {
	Text  string `json:"text"`
	Index int    `json:"index"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// ChunkOptions configures how text is split into chunks.
type ChunkOptions struct {
	ChunkSize int `json:"chunk_size,omitempty"`
	Overlap   int `json:"overlap,omitempty"`
}

// Config holds RAG pack configuration.
type Config struct {
	// Embedder generates vector embeddings (required).
	Embedder Embedder

	// Store provides vector storage and search (required).
	Store VectorStore

	// Reranker optionally reorders results for better relevance.
	Reranker Reranker

	// Chunker optionally splits documents into chunks.
	// If nil, a simple sentence-based chunker is used.
	Chunker Chunker

	// DefaultChunkSize is the default chunk size in characters.
	DefaultChunkSize int

	// DefaultChunkOverlap is the default overlap between chunks.
	DefaultChunkOverlap int

	// DefaultTopK is the default number of results to return.
	DefaultTopK int

	// DefaultIndex is the default index name.
	DefaultIndex string
}

// Pack returns the RAG pipeline tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &ragPack{cfg: cfg}
	if p.cfg.DefaultChunkSize == 0 {
		p.cfg.DefaultChunkSize = 512
	}
	if p.cfg.DefaultChunkOverlap == 0 {
		p.cfg.DefaultChunkOverlap = 64
	}
	if p.cfg.DefaultTopK == 0 {
		p.cfg.DefaultTopK = 10
	}
	if p.cfg.DefaultIndex == "" {
		p.cfg.DefaultIndex = "default"
	}
	if p.cfg.Chunker == nil {
		p.cfg.Chunker = &sentenceChunker{}
	}

	tools := []tool.Tool{
		p.chunkDocumentTool(),
		p.indexChunksTool(),
		p.retrieveTool(),
		p.hybridSearchTool(),
		p.deleteIndexTool(),
	}

	if cfg.Reranker != nil {
		tools = append(tools, p.rerankTool())
	}

	return pack.NewBuilder("rag").
		WithDescription("Retrieval-Augmented Generation pipeline tools for chunking, indexing, search, and reranking").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type ragPack struct {
	cfg Config
}

// ============================================================================
// Pipeline Tools
// ============================================================================

func (p *ragPack) chunkDocumentTool() tool.Tool {
	return tool.NewBuilder("rag_chunk_document").
		WithDescription("Split a document into chunks suitable for embedding and indexing").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Text      string `json:"text"`
				ChunkSize int    `json:"chunk_size,omitempty"`
				Overlap   int    `json:"overlap,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Text == "" {
				return tool.Result{}, fmt.Errorf("text is required")
			}

			chunkSize := in.ChunkSize
			if chunkSize == 0 {
				chunkSize = p.cfg.DefaultChunkSize
			}
			overlap := in.Overlap
			if overlap == 0 {
				overlap = p.cfg.DefaultChunkOverlap
			}

			chunks, err := p.cfg.Chunker.Chunk(ctx, in.Text, ChunkOptions{
				ChunkSize: chunkSize,
				Overlap:   overlap,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("chunking failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":      len(chunks),
				"chunk_size": chunkSize,
				"overlap":    overlap,
				"chunks":     chunks,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ragPack) indexChunksTool() tool.Tool {
	return tool.NewBuilder("rag_index_chunks").
		WithDescription("Embed and index document chunks into the vector store").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Index    string         `json:"index,omitempty"`
				Chunks   []string       `json:"chunks"`
				Metadata map[string]any `json:"metadata,omitempty"`
				IDPrefix string         `json:"id_prefix,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if len(in.Chunks) == 0 {
				return tool.Result{}, fmt.Errorf("chunks is required")
			}

			index := in.Index
			if index == "" {
				index = p.cfg.DefaultIndex
			}

			prefix := in.IDPrefix
			if prefix == "" {
				prefix = "chunk"
			}

			// Embed all chunks in batch
			vectors, err := p.cfg.Embedder.EmbedBatch(ctx, in.Chunks)
			if err != nil {
				return tool.Result{}, fmt.Errorf("embedding failed: %w", err)
			}

			// Build documents
			docs := make([]Document, len(in.Chunks))
			for i, text := range in.Chunks {
				meta := make(map[string]any)
				for k, v := range in.Metadata {
					meta[k] = v
				}
				meta["chunk_index"] = i
				meta["text_length"] = len(text)

				docs[i] = Document{
					ID:       fmt.Sprintf("%s-%d", prefix, i),
					Text:     text,
					Vector:   vectors[i],
					Metadata: meta,
				}
			}

			// Store in vector store
			if err := p.cfg.Store.UpsertBatch(ctx, index, docs); err != nil {
				return tool.Result{}, fmt.Errorf("indexing failed: %w", err)
			}

			dims := 0
			if len(vectors) > 0 {
				dims = len(vectors[0])
			}

			output, _ := json.Marshal(map[string]any{
				"index":      index,
				"count":      len(docs),
				"dimensions": dims,
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ragPack) retrieveTool() tool.Tool {
	return tool.NewBuilder("rag_retrieve").
		WithDescription("Retrieve relevant document chunks for a query using vector similarity search").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query  string         `json:"query"`
				Index  string         `json:"index,omitempty"`
				TopK   int            `json:"top_k,omitempty"`
				Filter map[string]any `json:"filter,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}

			index := in.Index
			if index == "" {
				index = p.cfg.DefaultIndex
			}
			topK := in.TopK
			if topK == 0 {
				topK = p.cfg.DefaultTopK
			}

			// Embed the query
			queryVec, err := p.cfg.Embedder.Embed(ctx, in.Query)
			if err != nil {
				return tool.Result{}, fmt.Errorf("query embedding failed: %w", err)
			}

			// Search the vector store
			results, err := p.cfg.Store.Search(ctx, index, queryVec, topK, in.Filter)
			if err != nil {
				return tool.Result{}, fmt.Errorf("search failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"query":   in.Query,
				"index":   index,
				"count":   len(results),
				"results": results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ragPack) hybridSearchTool() tool.Tool {
	return tool.NewBuilder("rag_hybrid_search").
		WithDescription("Combine vector similarity and keyword matching for improved retrieval").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query        string         `json:"query"`
				Index        string         `json:"index,omitempty"`
				TopK         int            `json:"top_k,omitempty"`
				Filter       map[string]any `json:"filter,omitempty"`
				VectorWeight float64        `json:"vector_weight,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}

			index := in.Index
			if index == "" {
				index = p.cfg.DefaultIndex
			}
			topK := in.TopK
			if topK == 0 {
				topK = p.cfg.DefaultTopK
			}
			vectorWeight := in.VectorWeight
			if vectorWeight == 0 {
				vectorWeight = 0.7
			}
			keywordWeight := 1.0 - vectorWeight

			// Vector search
			queryVec, err := p.cfg.Embedder.Embed(ctx, in.Query)
			if err != nil {
				return tool.Result{}, fmt.Errorf("query embedding failed: %w", err)
			}

			// Fetch more results for fusion
			fetchK := topK * 3
			vectorResults, err := p.cfg.Store.Search(ctx, index, queryVec, fetchK, in.Filter)
			if err != nil {
				return tool.Result{}, fmt.Errorf("vector search failed: %w", err)
			}

			// Keyword scoring via BM25-like term frequency
			queryTerms := tokenize(in.Query)
			scored := make(map[string]*fusionScore)

			// Score vector results
			for rank, doc := range vectorResults {
				rrf := 1.0 / float64(rank+60) // reciprocal rank fusion constant k=60
				s, ok := scored[doc.ID]
				if !ok {
					s = &fusionScore{doc: doc}
					scored[doc.ID] = s
				}
				s.vectorScore = rrf
				s.vectorRank = rank
			}

			// Score keyword results
			for id, s := range scored {
				_ = id
				kwScore := keywordScore(s.doc.Text, queryTerms)
				s.keywordScore = kwScore
			}

			// Combine scores
			var fused []ScoredDocument
			for _, s := range scored {
				combined := vectorWeight*s.vectorScore + keywordWeight*s.keywordScore
				fused = append(fused, ScoredDocument{
					Document: s.doc.Document,
					Score:    combined,
				})
			}

			sort.Slice(fused, func(i, j int) bool {
				return fused[i].Score > fused[j].Score
			})

			if len(fused) > topK {
				fused = fused[:topK]
			}

			output, _ := json.Marshal(map[string]any{
				"query":         in.Query,
				"index":         index,
				"count":         len(fused),
				"vector_weight": vectorWeight,
				"results":       fused,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ragPack) rerankTool() tool.Tool {
	return tool.NewBuilder("rag_rerank").
		WithDescription("Rerank retrieved documents using a cross-encoder model for improved relevance").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Query string           `json:"query"`
				Docs  []ScoredDocument `json:"documents"`
				TopN  int              `json:"top_n,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Query == "" {
				return tool.Result{}, fmt.Errorf("query is required")
			}
			if len(in.Docs) == 0 {
				return tool.Result{}, fmt.Errorf("documents is required")
			}

			topN := in.TopN
			if topN == 0 {
				topN = len(in.Docs)
			}

			reranked, err := p.cfg.Reranker.Rerank(ctx, in.Query, in.Docs, topN)
			if err != nil {
				return tool.Result{}, fmt.Errorf("reranking failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"query":   in.Query,
				"count":   len(reranked),
				"results": reranked,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *ragPack) deleteIndexTool() tool.Tool {
	return tool.NewBuilder("rag_delete_index").
		WithDescription("Delete an entire index and all its documents from the vector store").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Index string `json:"index"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Index == "" {
				return tool.Result{}, fmt.Errorf("index is required")
			}

			if err := p.cfg.Store.DeleteIndex(ctx, in.Index); err != nil {
				return tool.Result{}, fmt.Errorf("delete index failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"index":   in.Index,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Built-in Chunker
// ============================================================================

// sentenceChunker splits text on sentence boundaries.
type sentenceChunker struct{}

func (c *sentenceChunker) Chunk(_ context.Context, text string, opts ChunkOptions) ([]Chunk, error) {
	if text == "" {
		return nil, nil
	}

	chunkSize := opts.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 512
	}
	overlap := opts.Overlap
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	sentences := splitSentences(text)
	var chunks []Chunk
	var current strings.Builder
	startPos := 0
	idx := 0

	for _, sent := range sentences {
		if current.Len()+len(sent) > chunkSize && current.Len() > 0 {
			chunks = append(chunks, Chunk{
				Text:  current.String(),
				Index: idx,
				Start: startPos,
				End:   startPos + current.Len(),
			})
			idx++

			// Handle overlap by keeping trailing text
			if overlap > 0 {
				full := current.String()
				overlapStart := len(full) - overlap
				if overlapStart < 0 {
					overlapStart = 0
				}
				startPos = startPos + overlapStart
				current.Reset()
				current.WriteString(full[overlapStart:])
			} else {
				startPos += current.Len()
				current.Reset()
			}
		}
		current.WriteString(sent)
	}

	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			Text:  current.String(),
			Index: idx,
			Start: startPos,
			End:   startPos + current.Len(),
		})
	}

	return chunks, nil
}

// splitSentences splits text into sentence-like segments.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, r := range text {
		current.WriteRune(r)
		if (r == '.' || r == '!' || r == '?') && i+1 < len(text) {
			next := rune(text[i+1])
			if next == ' ' || next == '\n' || next == '\r' {
				sentences = append(sentences, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}

	return sentences
}

// ============================================================================
// Hybrid Search Helpers
// ============================================================================

type fusionScore struct {
	doc          ScoredDocument
	vectorScore  float64
	vectorRank   int
	keywordScore float64
}

// tokenize splits text into lowercase terms.
func tokenize(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	})
	return words
}

// keywordScore computes a simple BM25-like relevance score.
func keywordScore(text string, queryTerms []string) float64 {
	if len(queryTerms) == 0 || text == "" {
		return 0
	}

	textLower := strings.ToLower(text)
	textTerms := tokenize(textLower)

	// Term frequency map
	tf := make(map[string]int)
	for _, t := range textTerms {
		tf[t]++
	}

	docLen := float64(len(textTerms))
	avgDL := 256.0 // assumed average document length
	k1 := 1.2
	b := 0.75

	var score float64
	for _, qt := range queryTerms {
		freq := float64(tf[qt])
		if freq == 0 {
			continue
		}
		// Simplified BM25 (without IDF since we score single docs)
		numerator := freq * (k1 + 1)
		denominator := freq + k1*(1-b+b*(docLen/avgDL))
		score += numerator / denominator
	}

	// Normalize to 0-1 range (approximate)
	maxScore := float64(len(queryTerms)) * (k1 + 1)
	if maxScore > 0 {
		score = math.Min(score/maxScore, 1.0)
	}

	return score
}

// Ensure sentenceChunker implements Chunker.
var _ Chunker = (*sentenceChunker)(nil)

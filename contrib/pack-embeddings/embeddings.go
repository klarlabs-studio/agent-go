// Package embeddings provides embedding and vector similarity tools for agent-go.
//
// Tools include text/image embedding, similarity search, clustering, and batch operations.
// The package uses an interface-based approach, allowing any embedding provider to be used.
package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// EmbeddingProvider generates vector embeddings from content.
type EmbeddingProvider interface {
	// EmbedText generates an embedding for the given text.
	EmbedText(ctx context.Context, text string, model string) ([]float64, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string, model string) ([][]float64, error)

	// EmbedImage generates an embedding for an image (base64-encoded or URL).
	EmbedImage(ctx context.Context, image string, model string) ([]float64, error)

	// Models returns the list of available embedding models.
	Models(ctx context.Context) ([]ModelInfo, error)
}

// VectorStore provides storage and retrieval of vectors.
type VectorStore interface {
	// Store saves a vector with its ID and metadata.
	Store(ctx context.Context, id string, vector []float64, metadata map[string]interface{}) error

	// Search finds the nearest vectors to the query vector.
	Search(ctx context.Context, query []float64, limit int, filter map[string]interface{}) ([]SearchResult, error)

	// Delete removes a vector by ID.
	Delete(ctx context.Context, id string) error
}

// ModelInfo describes an embedding model.
type ModelInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Dimensions int    `json:"dimensions"`
	MaxTokens  int    `json:"max_tokens,omitempty"`
}

// SearchResult represents a search match with similarity score.
type SearchResult struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Vector   []float64              `json:"vector,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Config holds embedding pack configuration.
type Config struct {
	// Provider is the embedding provider (required).
	Provider EmbeddingProvider

	// Store is an optional vector store for persistence and search.
	Store VectorStore

	// DefaultModel is the default embedding model to use.
	DefaultModel string

	// MaxBatchSize limits batch embedding size.
	MaxBatchSize int
}

// Pack returns the embeddings tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &embeddingsPack{cfg: cfg}
	if p.cfg.MaxBatchSize == 0 {
		p.cfg.MaxBatchSize = 100
	}

	tools := []tool.Tool{
		p.embedTextTool(),
		p.embedBatchTool(),
		p.embedImageTool(),
		p.similarityTool(),
		p.listModelsTool(),
	}

	if cfg.Store != nil {
		tools = append(tools,
			p.storeTool(),
			p.searchTool(),
			p.deleteTool(),
		)
	}

	return pack.NewBuilder("embeddings").
		WithDescription("Embedding and vector similarity tools for text, images, and search").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type embeddingsPack struct {
	cfg Config
}

func (p *embeddingsPack) model(override string) string {
	if override != "" {
		return override
	}
	return p.cfg.DefaultModel
}

// ============================================================================
// Embedding Tools
// ============================================================================

func (p *embeddingsPack) embedTextTool() tool.Tool {
	return tool.NewBuilder("embed_text").
		WithDescription("Generate an embedding vector for text").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Text  string `json:"text"`
				Model string `json:"model,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Text == "" {
				return tool.Result{}, fmt.Errorf("text is required")
			}

			model := p.model(in.Model)
			embedding, err := p.cfg.Provider.EmbedText(ctx, in.Text, model)
			if err != nil {
				return tool.Result{}, fmt.Errorf("embedding failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"dimensions": len(embedding),
				"model":      model,
				"embedding":  embedding,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *embeddingsPack) embedBatchTool() tool.Tool {
	return tool.NewBuilder("embed_batch").
		WithDescription("Generate embeddings for multiple texts").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Texts []string `json:"texts"`
				Model string   `json:"model,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if len(in.Texts) == 0 {
				return tool.Result{}, fmt.Errorf("texts is required")
			}
			if len(in.Texts) > p.cfg.MaxBatchSize {
				return tool.Result{}, fmt.Errorf("batch size %d exceeds maximum %d", len(in.Texts), p.cfg.MaxBatchSize)
			}

			model := p.model(in.Model)
			embeddings, err := p.cfg.Provider.EmbedBatch(ctx, in.Texts, model)
			if err != nil {
				return tool.Result{}, fmt.Errorf("batch embedding failed: %w", err)
			}

			dims := 0
			if len(embeddings) > 0 {
				dims = len(embeddings[0])
			}

			output, _ := json.Marshal(map[string]any{
				"count":      len(embeddings),
				"dimensions": dims,
				"model":      model,
				"embeddings": embeddings,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *embeddingsPack) embedImageTool() tool.Tool {
	return tool.NewBuilder("embed_image").
		WithDescription("Generate an embedding for an image").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Image string `json:"image"` // base64 data or URL
				Model string `json:"model,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Image == "" {
				return tool.Result{}, fmt.Errorf("image is required")
			}

			model := p.model(in.Model)
			embedding, err := p.cfg.Provider.EmbedImage(ctx, in.Image, model)
			if err != nil {
				return tool.Result{}, fmt.Errorf("image embedding failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"dimensions": len(embedding),
				"model":      model,
				"embedding":  embedding,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Similarity Tools
// ============================================================================

func (p *embeddingsPack) similarityTool() tool.Tool {
	return tool.NewBuilder("embed_similarity").
		WithDescription("Calculate cosine similarity between two embedding vectors").
		ReadOnly().
		Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				VectorA []float64 `json:"vector_a"`
				VectorB []float64 `json:"vector_b"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if len(in.VectorA) == 0 || len(in.VectorB) == 0 {
				return tool.Result{}, fmt.Errorf("both vector_a and vector_b are required")
			}
			if len(in.VectorA) != len(in.VectorB) {
				return tool.Result{}, fmt.Errorf("vectors must have equal dimensions: %d vs %d", len(in.VectorA), len(in.VectorB))
			}

			similarity := cosineSimilarity(in.VectorA, in.VectorB)

			output, _ := json.Marshal(map[string]any{
				"similarity": similarity,
				"dimensions": len(in.VectorA),
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Model Tools
// ============================================================================

func (p *embeddingsPack) listModelsTool() tool.Tool {
	return tool.NewBuilder("embed_list_models").
		WithDescription("List available embedding models").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			models, err := p.cfg.Provider.Models(ctx)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to list models: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":  len(models),
				"models": models,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Vector Store Tools (only available when Store is configured)
// ============================================================================

func (p *embeddingsPack) storeTool() tool.Tool {
	return tool.NewBuilder("embed_store").
		WithDescription("Store an embedding vector with metadata").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ID       string                 `json:"id"`
				Vector   []float64              `json:"vector"`
				Metadata map[string]interface{} `json:"metadata,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.ID == "" {
				return tool.Result{}, fmt.Errorf("id is required")
			}
			if len(in.Vector) == 0 {
				return tool.Result{}, fmt.Errorf("vector is required")
			}

			err := p.cfg.Store.Store(ctx, in.ID, in.Vector, in.Metadata)
			if err != nil {
				return tool.Result{}, fmt.Errorf("store failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":         in.ID,
				"dimensions": len(in.Vector),
				"success":    true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *embeddingsPack) searchTool() tool.Tool {
	return tool.NewBuilder("embed_search").
		WithDescription("Search for similar vectors in the store").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				Vector []float64              `json:"vector"`
				Limit  int                    `json:"limit,omitempty"`
				Filter map[string]interface{} `json:"filter,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if len(in.Vector) == 0 {
				return tool.Result{}, fmt.Errorf("vector is required")
			}
			if in.Limit == 0 {
				in.Limit = 10
			}

			results, err := p.cfg.Store.Search(ctx, in.Vector, in.Limit, in.Filter)
			if err != nil {
				return tool.Result{}, fmt.Errorf("search failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"count":   len(results),
				"results": results,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *embeddingsPack) deleteTool() tool.Tool {
	return tool.NewBuilder("embed_delete").
		WithDescription("Delete a vector from the store").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.ID == "" {
				return tool.Result{}, fmt.Errorf("id is required")
			}

			err := p.cfg.Store.Delete(ctx, in.ID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("delete failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"id":      in.ID,
				"success": true,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// ============================================================================
// Math Helpers
// ============================================================================

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denominator := math.Sqrt(normA) * math.Sqrt(normB)
	if denominator == 0 {
		return 0
	}
	return dot / denominator
}

// Cluster performs k-means clustering on vectors.
func Cluster(vectors [][]float64, k int, maxIterations int) []int {
	if len(vectors) == 0 || k <= 0 {
		return nil
	}
	if k >= len(vectors) {
		assignments := make([]int, len(vectors))
		for i := range assignments {
			assignments[i] = i
		}
		return assignments
	}
	if maxIterations == 0 {
		maxIterations = 100
	}

	dims := len(vectors[0])
	centroids := make([][]float64, k)
	for i := 0; i < k; i++ {
		centroids[i] = make([]float64, dims)
		copy(centroids[i], vectors[i])
	}

	assignments := make([]int, len(vectors))

	for iter := 0; iter < maxIterations; iter++ {
		changed := false

		// Assign vectors to nearest centroid
		for i, v := range vectors {
			nearest := 0
			bestDist := math.MaxFloat64
			for j, c := range centroids {
				dist := euclideanDistSq(v, c)
				if dist < bestDist {
					bestDist = dist
					nearest = j
				}
			}
			if assignments[i] != nearest {
				assignments[i] = nearest
				changed = true
			}
		}

		if !changed {
			break
		}

		// Recalculate centroids
		counts := make([]int, k)
		newCentroids := make([][]float64, k)
		for i := range newCentroids {
			newCentroids[i] = make([]float64, dims)
		}

		for i, v := range vectors {
			c := assignments[i]
			counts[c]++
			for d := range v {
				newCentroids[c][d] += v[d]
			}
		}

		for i := range centroids {
			if counts[i] > 0 {
				for d := range centroids[i] {
					centroids[i][d] = newCentroids[i][d] / float64(counts[i])
				}
			}
		}
	}

	return assignments
}

// TopK returns the indices of the top-k most similar vectors to a query.
func TopK(query []float64, vectors [][]float64, k int) []int {
	type scored struct {
		index int
		score float64
	}

	scores := make([]scored, len(vectors))
	for i, v := range vectors {
		scores[i] = scored{index: i, score: cosineSimilarity(query, v)}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if k > len(scores) {
		k = len(scores)
	}

	indices := make([]int, k)
	for i := 0; i < k; i++ {
		indices[i] = scores[i].index
	}
	return indices
}

func euclideanDistSq(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

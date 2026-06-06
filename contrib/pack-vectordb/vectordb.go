// Package vectordb provides vector database tools for agent-go.
//
// This pack includes tools for vector database operations:
//   - vector_upsert: Insert or update vectors
//   - vector_query: Query for similar vectors
//   - vector_delete: Delete vectors by ID
//   - vector_fetch: Fetch vectors by ID
//   - vector_list: List vectors in an index
//   - vector_create_index: Create a new vector index
//   - vector_delete_index: Delete a vector index
//   - vector_describe_index: Get index statistics
//
// Supports Pinecone, Weaviate, Milvus, Qdrant, and pgvector.
package vectordb

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the vector database tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("vectordb").
		WithDescription("Vector database tools for similarity search").
		WithVersion("0.1.0").
		AddTools(
			vectorUpsert(),
			vectorQuery(),
			vectorDelete(),
			vectorFetch(),
			vectorList(),
			vectorCreateIndex(),
			vectorDeleteIndex(),
			vectorDescribeIndex(),
		).
		AllowInState(agent.StateExplore, "vector_query", "vector_fetch", "vector_list", "vector_describe_index").
		AllowInState(agent.StateAct, "vector_upsert", "vector_query", "vector_delete", "vector_fetch", "vector_list", "vector_create_index", "vector_delete_index", "vector_describe_index").
		AllowInState(agent.StateValidate, "vector_query", "vector_fetch", "vector_describe_index").
		Build()
}

func vectorUpsert() tool.Tool {
	return tool.NewBuilder("vector_upsert").
		WithDescription("Insert or update vectors with metadata").
		Idempotent().
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func vectorQuery() tool.Tool {
	return tool.NewBuilder("vector_query").
		WithDescription("Query for similar vectors using vector similarity search").
		ReadOnly().
		MustBuild()
}

func vectorDelete() tool.Tool {
	return tool.NewBuilder("vector_delete").
		WithDescription("Delete vectors by ID or filter").
		Destructive().
		MustBuild()
}

func vectorFetch() tool.Tool {
	return tool.NewBuilder("vector_fetch").
		WithDescription("Fetch vectors by their IDs").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func vectorList() tool.Tool {
	return tool.NewBuilder("vector_list").
		WithDescription("List vectors in an index with pagination").
		ReadOnly().
		MustBuild()
}

func vectorCreateIndex() tool.Tool {
	return tool.NewBuilder("vector_create_index").
		WithDescription("Create a new vector index").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func vectorDeleteIndex() tool.Tool {
	return tool.NewBuilder("vector_delete_index").
		WithDescription("Delete a vector index").
		Destructive().
		MustBuild()
}

func vectorDescribeIndex() tool.Tool {
	return tool.NewBuilder("vector_describe_index").
		WithDescription("Get statistics and configuration for a vector index").
		ReadOnly().
		Cacheable().
		MustBuild()
}

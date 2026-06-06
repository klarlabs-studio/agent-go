// Package search provides search tools for agent-go.
//
// This pack includes tools for search operations:
//   - search_query: Execute a search query
//   - search_index: Index a document
//   - search_bulk_index: Bulk index multiple documents
//   - search_delete: Delete a document from the index
//   - search_suggest: Get search suggestions/autocomplete
//   - search_aggregate: Run aggregation queries
//   - search_indices: List available indices
//
// Supports Elasticsearch, OpenSearch, Meilisearch, and Typesense.
package search

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the search tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("search").
		WithDescription("Search engine tools for indexing and querying").
		WithVersion("0.1.0").
		AddTools(
			searchQuery(),
			searchIndex(),
			searchBulkIndex(),
			searchDelete(),
			searchSuggest(),
			searchAggregate(),
			searchIndices(),
		).
		AllowInState(agent.StateExplore, "search_query", "search_suggest", "search_aggregate", "search_indices").
		AllowInState(agent.StateAct, "search_query", "search_index", "search_bulk_index", "search_delete", "search_suggest", "search_aggregate", "search_indices").
		AllowInState(agent.StateValidate, "search_query", "search_indices").
		Build()
}

func searchQuery() tool.Tool {
	return tool.NewBuilder("search_query").
		WithDescription("Execute a search query and return matching documents").
		ReadOnly().
		MustBuild()
}

func searchIndex() tool.Tool {
	return tool.NewBuilder("search_index").
		WithDescription("Index a document for searching").
		Idempotent().
		WithRiskLevel(tool.RiskLow).
		MustBuild()
}

func searchBulkIndex() tool.Tool {
	return tool.NewBuilder("search_bulk_index").
		WithDescription("Bulk index multiple documents").
		WithRiskLevel(tool.RiskMedium).
		MustBuild()
}

func searchDelete() tool.Tool {
	return tool.NewBuilder("search_delete").
		WithDescription("Delete a document from the search index").
		Destructive().
		MustBuild()
}

func searchSuggest() tool.Tool {
	return tool.NewBuilder("search_suggest").
		WithDescription("Get search suggestions and autocomplete results").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func searchAggregate() tool.Tool {
	return tool.NewBuilder("search_aggregate").
		WithDescription("Run aggregation queries for analytics").
		ReadOnly().
		MustBuild()
}

func searchIndices() tool.Tool {
	return tool.NewBuilder("search_indices").
		WithDescription("List available search indices").
		ReadOnly().
		Cacheable().
		MustBuild()
}

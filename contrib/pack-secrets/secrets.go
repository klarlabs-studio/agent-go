// Package secrets provides secret management tools for agent-go.
//
// This pack includes tools for secret management:
//   - secrets_get: Retrieve a secret by key
//   - secrets_set: Store a secret
//   - secrets_delete: Delete a secret
//   - secrets_list: List available secrets (keys only)
//   - secrets_rotate: Rotate a secret value
//   - secrets_version: Get a specific version of a secret
//
// Supports HashiCorp Vault, AWS Secrets Manager, GCP Secret Manager,
// Azure Key Vault, and local encrypted storage.
package secrets

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack returns the secrets management tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("secrets").
		WithDescription("Secret management tools for secure credential storage").
		WithVersion("0.1.0").
		AddTools(
			secretsGet(),
			secretsSet(),
			secretsDelete(),
			secretsList(),
			secretsRotate(),
			secretsVersion(),
		).
		AllowInState(agent.StateExplore, "secrets_list").
		AllowInState(agent.StateAct, "secrets_get", "secrets_set", "secrets_delete", "secrets_list", "secrets_rotate", "secrets_version").
		Build()
}

func secretsGet() tool.Tool {
	return tool.NewBuilder("secrets_get").
		WithDescription("Retrieve a secret value by key").
		ReadOnly().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func secretsSet() tool.Tool {
	return tool.NewBuilder("secrets_set").
		WithDescription("Store or update a secret").
		Idempotent().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func secretsDelete() tool.Tool {
	return tool.NewBuilder("secrets_delete").
		WithDescription("Delete a secret").
		Destructive().
		MustBuild()
}

func secretsList() tool.Tool {
	return tool.NewBuilder("secrets_list").
		WithDescription("List available secret keys (not values)").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func secretsRotate() tool.Tool {
	return tool.NewBuilder("secrets_rotate").
		WithDescription("Rotate a secret to a new value").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func secretsVersion() tool.Tool {
	return tool.NewBuilder("secrets_version").
		WithDescription("Get a specific version of a secret").
		ReadOnly().
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

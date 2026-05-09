package state

import "github.com/mistypass/cloud/api/internal/state/migrations"

// migrations_registry.go wires individual migration files into the
// Migration slice returned by AllMigrations().

func migrations001InitialSchema() Migration {
	return Migration{
		Version:     1,
		Description: "initial schema baseline",
		Up:          migrations.InitialSchemaUp,
	}
}

func migrations002MultiOrg() Migration {
	return Migration{
		Version:     2,
		Description: "multi-org memberships and magic link tokens",
		Up:          migrations.MultiOrgUp,
	}
}

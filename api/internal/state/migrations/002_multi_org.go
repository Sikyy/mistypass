package migrations

import "database/sql"

func MultiOrgUp(tx *sql.Tx) error {
	// Create user_org_memberships table
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS user_org_memberships (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'resident',
			joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			last_used_at TIMESTAMPTZ,
			UNIQUE(user_id, tenant_id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_org_memberships_user ON user_org_memberships(user_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_org_memberships_tenant ON user_org_memberships(tenant_id)`)
	if err != nil {
		return err
	}

	// Create magic_link_tokens table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS magic_link_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_token ON magic_link_tokens(token)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_email ON magic_link_tokens(email)`)
	if err != nil {
		return err
	}

	// Backfill: create org membership for every existing user
	_, err = tx.Exec(`
		INSERT INTO user_org_memberships (user_id, tenant_id, role)
		SELECT id, tenant_id, role FROM mistypass_auth_users
		ON CONFLICT (user_id, tenant_id) DO NOTHING
	`)
	return err
}

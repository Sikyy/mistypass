package state

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestDeleteProjectionRowsNotInIDsRejectsUnknownTable(t *testing.T) {
	err := deleteProjectionRowsNotInIDs(context.Background(), (*sql.Tx)(nil), "mistypass_unknown_table", nil)
	if err == nil {
		t.Fatalf("expected invalid projection table error")
	}
	if !errors.Is(err, ErrInvalidProjectionTable) {
		t.Fatalf("expected ErrInvalidProjectionTable, got %v", err)
	}
}

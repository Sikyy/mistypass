package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

func TestEnterpriseTrustedSessionErrorStatusCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "inactive employee forbidden",
			err:  enterprise.ErrEmployeeInactive,
			want: http.StatusForbidden,
		},
		{
			name: "external id conflict",
			err:  enterprise.ErrEmployeeExternalIDConflict,
			want: http.StatusConflict,
		},
		{
			name: "jit approval required forbidden",
			err:  enterprise.ErrJITProvisionApprovalRequired,
			want: http.StatusForbidden,
		},
		{
			name: "email required bad request",
			err:  enterprise.ErrEmailRequired,
			want: http.StatusBadRequest,
		},
		{
			name: "invalid domain bad request",
			err:  enterprise.ErrInvalidDomain,
			want: http.StatusBadRequest,
		},
		{
			name: "invalid credentials unauthorized",
			err:  auth.ErrInvalidCredentials,
			want: http.StatusUnauthorized,
		},
		{
			name: "wrapped inactive employee forbidden",
			err:  fmt.Errorf("wrapped: %w", enterprise.ErrEmployeeInactive),
			want: http.StatusForbidden,
		},
		{
			name: "unknown internal server error",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := enterpriseTrustedSessionErrorStatusCode(tt.err)
			if got != tt.want {
				t.Fatalf("expected status=%d, got=%d", tt.want, got)
			}
		})
	}
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/storage"
)

// NewProgramAuth constructs the service.IProgramAuth implementation (API keys + scopes).
func NewProgramAuth(
	state storage.ISystemAuthStateRepository,
	keys storage.IAPIKeyRepository,
	audit storage.IAPIKeyAuditRepository,
) service.IProgramAuth {
	return &programAuth{
		state: state,
		keys:  keys,
		audit: audit,
	}
}

type programAuth struct {
	state storage.ISystemAuthStateRepository
	keys  storage.IAPIKeyRepository
	audit storage.IAPIKeyAuditRepository
}

func (s *programAuth) IsSystemInitialized(ctx context.Context) (bool, error) {
	st, err := s.state.Get(ctx)
	if err != nil {
		return false, err
	}
	return st.InitializedAt != nil && !st.InitializedAt.IsZero(), nil
}

func (s *programAuth) ValidateAPIKey(ctx context.Context, bearerToken string) (*service.APIKeyPrincipal, error) {
	bearerToken = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(bearerToken), "Bearer "))
	if bearerToken == "" {
		return nil, service.ErrAuthInvalidAPIKey
	}
	h := sha256Hex(bearerToken)
	k, err := s.keys.GetByKeyHash(ctx, h)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAuthInvalidAPIKey
		}
		return nil, err
	}
	if k.RevokedAt != nil {
		return nil, service.ErrAuthAPIKeyRevoked
	}
	now := time.Now().UTC()
	if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
		return nil, service.ErrAuthInvalidAPIKey
	}
	return &service.APIKeyPrincipal{KeyID: k.ID, Scope: k.Scope, Name: k.Name}, nil
}

func (s *programAuth) APIKeyMeetsScope(principal *service.APIKeyPrincipal, requiredScope string) bool {
	if principal == nil {
		return false
	}
	return apiKeyScopeRank(principal.Scope) >= apiKeyScopeRank(requiredScope)
}

func (s *programAuth) RecordAPIKeyUse(ctx context.Context, keyID, method, path, clientIP string) error {
	now := time.Now().UTC()
	if err := s.audit.Insert(ctx, keyID, method, path, clientIP, now); err != nil {
		return err
	}
	return s.keys.TouchLastUsed(ctx, keyID, clientIP, now)
}

var _ service.IProgramAuth = (*programAuth)(nil)

package svcimpl

import (
	"context"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// AdminConfigViewSvc implements service.IAdminConfigViewService (stateless; reads global viper).
type AdminConfigViewSvc struct{}

// NewAdminConfigViewSvc constructs the admin config snapshot service.
func NewAdminConfigViewSvc() *AdminConfigViewSvc {
	return &AdminConfigViewSvc{}
}

var _ service.IAdminConfigViewService = (*AdminConfigViewSvc)(nil)

func (s *AdminConfigViewSvc) RedactedSnapshot(ctx context.Context, actor *service.BrowserPrincipal) (map[string]interface{}, error) {
	if actor == nil || actor.Role != types.AuthRoleAdmin {
		return nil, service.ErrAuthForbidden
	}
	_ = ctx
	return redactConfigMap(viper.AllSettings()), nil
}

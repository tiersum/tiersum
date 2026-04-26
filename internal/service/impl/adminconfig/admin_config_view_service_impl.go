package adminconfig

import (
	"context"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewAdminConfigViewService constructs a service.IAdminConfigViewService implementation backed by viper.
func NewAdminConfigViewService() service.IAdminConfigViewService {
	return &adminConfigViewService{}
}

type adminConfigViewService struct{}

func (s *adminConfigViewService) RedactedSnapshot(ctx context.Context, actor *service.BrowserPrincipal) (map[string]interface{}, error) {
	if actor == nil || actor.Role != types.AuthRoleAdmin {
		return nil, service.ErrAuthForbidden
	}
	_ = ctx
	return RedactConfigMap(viper.AllSettings()), nil
}

var _ service.IAdminConfigViewService = (*adminConfigViewService)(nil)

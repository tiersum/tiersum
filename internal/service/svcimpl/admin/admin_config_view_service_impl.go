package admin

import (
	"context"

	"github.com/spf13/viper"

	"github.com/tiersum/tiersum/internal/service"
	"github.com/tiersum/tiersum/internal/service/svcimpl/common"
	"github.com/tiersum/tiersum/pkg/types"
)

// NewAdminConfigViewService constructs the service.IAdminConfigViewService implementation.
func NewAdminConfigViewService() service.IAdminConfigViewService {
	return &adminConfigViewService{}
}

type adminConfigViewService struct{}

func (s *adminConfigViewService) RedactedSnapshot(ctx context.Context, actor *service.BrowserPrincipal) (map[string]interface{}, error) {
	if actor == nil || actor.Role != types.AuthRoleAdmin {
		return nil, service.ErrAuthForbidden
	}
	_ = ctx
	return common.RedactConfigMap(viper.AllSettings()), nil
}

var _ service.IAdminConfigViewService = (*adminConfigViewService)(nil)

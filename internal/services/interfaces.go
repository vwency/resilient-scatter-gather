package services

import (
	"context"

	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type UserService interface {
	GetUser(ctx context.Context, userID string) (*pb_user.GetUserResponse, error)
}

type PermissionsService interface {
	CheckAccess(ctx context.Context, userID, resourceID string) (*pb_permissions.CheckAccessResponse, error)
}

type VectorMemoryService interface {
	GetContext(ctx context.Context, chatID string) (*pb_vector.GetContextResponse, error)
}

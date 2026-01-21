package handler_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/valyala/fasthttp"
	"github.com/vwency/resilient-scatter-gather/internal/handler"
	"github.com/vwency/resilient-scatter-gather/internal/models"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

func TestHandle_UserServiceError_ReturnsInternalServerError(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	mockUser.On("GetUser", mock.Anything, "user123").Return(nil, errors.New("user service down"))

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil)

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{}}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	ctx := &fasthttp.RequestCtx{}
	ctx.QueryArgs().Add("user_id", "user123")
	ctx.QueryArgs().Add("chat_id", "chat1")

	h.Handle(ctx)

	assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())

	var errResponse models.ErrorResponse
	err := json.Unmarshal(ctx.Response.Body(), &errResponse)
	assert.NoError(t, err)
	assert.Equal(t, fasthttp.StatusInternalServerError, errResponse.Code)
	assert.Contains(t, errResponse.Message, "user")

	mockUser.AssertExpectations(t)
}

func TestHandle_UserServiceError_OtherServicesNotAffected(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	mockUser.On("GetUser", mock.Anything, "user123").Return(nil, errors.New("database connection failed")).Run(func(args mock.Arguments) {
		time.Sleep(5 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{
		Allowed:     true,
		Permissions: []string{"read"},
	}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "context"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	ctx := &fasthttp.RequestCtx{}
	ctx.QueryArgs().Add("user_id", "user123")
	ctx.QueryArgs().Add("chat_id", "chat1")

	h.Handle(ctx)

	assert.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode())

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

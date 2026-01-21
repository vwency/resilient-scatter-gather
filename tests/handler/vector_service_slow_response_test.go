package handler_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/valyala/fasthttp"
	"github.com/vwency/resilient-scatter-gather/internal/handler"
	"github.com/vwency/resilient-scatter-gather/internal/models"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

func TestHandle_VectorServiceSlowButSuccessful_ReturnsWithData(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	userResp := &pb_user.GetUserResponse{
		UserId:   "user123",
		Username: "testuser",
	}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{
		Allowed:     true,
		Permissions: []string{"read"},
	}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "slow but successful"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(130 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	ctx := &fasthttp.RequestCtx{}
	ctx.QueryArgs().Add("user_id", "user123")
	ctx.QueryArgs().Add("chat_id", "chat1")

	h.Handle(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	var response models.ChatSummaryResponse
	err := json.Unmarshal(ctx.Response.Body(), &response)
	assert.NoError(t, err)
	assert.NotNil(t, response.User)
	assert.NotNil(t, response.Permissions)
	assert.NotNil(t, response.Context)
	assert.False(t, response.Degraded)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestHandle_VectorServiceBarelySlow_ReturnsWithData(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "context"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(190 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	ctx := &fasthttp.RequestCtx{}
	ctx.QueryArgs().Add("user_id", "user123")
	ctx.QueryArgs().Add("chat_id", "chat1")

	start := time.Now()
	h.Handle(ctx)
	elapsed := time.Since(start)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Less(t, elapsed, 250*time.Millisecond)

	var response models.ChatSummaryResponse
	err := json.Unmarshal(ctx.Response.Body(), &response)
	assert.NoError(t, err)
	assert.NotNil(t, response.Context)
	assert.False(t, response.Degraded)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestHandle_VectorServiceVerySlowButNoTimeout_ReturnsWithData(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(5 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(30 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "very slow"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(160 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	ctx := &fasthttp.RequestCtx{}
	ctx.QueryArgs().Add("user_id", "user123")
	ctx.QueryArgs().Add("chat_id", "chat1")

	h.Handle(ctx)

	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	var response models.ChatSummaryResponse
	err := json.Unmarshal(ctx.Response.Body(), &response)
	assert.NoError(t, err)
	assert.NotNil(t, response.Context)
	assert.False(t, response.Degraded)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

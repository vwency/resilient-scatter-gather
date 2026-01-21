package handler_test

import (
	"context"
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

func TestHandle_UserServiceTimeout_ReturnsInternalServerError(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	mockUser.On("GetUser", mock.Anything, "user123").Return(nil, context.DeadlineExceeded).Run(func(args mock.Arguments) {
		time.Sleep(300 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{}}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

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

	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestHandle_UserServiceSlowButWithinSLA_ReturnsSuccess(t *testing.T) {
	mockUser := new(UserService)
	mockPermissions := new(PermissionsService)
	mockVector := new(VectorMemoryService)

	userResp := &pb_user.GetUserResponse{
		UserId:   "user123",
		Username: "testuser",
	}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(150 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(40 * time.Millisecond)
	})

	mockVector.On("GetContext", mock.Anything, "chat1").Return(nil, context.DeadlineExceeded).Run(func(args mock.Arguments) {
		time.Sleep(300 * time.Millisecond)
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
	assert.NotNil(t, response.User)
	assert.NotNil(t, response.Permissions)
	assert.Nil(t, response.Context)
	assert.True(t, response.Degraded)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
}

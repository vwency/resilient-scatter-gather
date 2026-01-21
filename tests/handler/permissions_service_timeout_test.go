package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vwency/resilient-scatter-gather/internal/handler"
	"github.com/vwency/resilient-scatter-gather/internal/models"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

func TestServeHTTP_PermissionsServiceTimeout_ReturnsInternalServerError(t *testing.T) {
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

	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(nil, context.DeadlineExceeded).Run(func(args mock.Arguments) {
		time.Sleep(300 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{}}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, errResponse.Code)

	mockUser.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestServeHTTP_PermissionsServiceSlowButWithinSLA_ReturnsSuccess(t *testing.T) {
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
		time.Sleep(180 * time.Millisecond)
	})

	mockVector.On("GetContext", mock.Anything, "chat1").Return(nil, context.DeadlineExceeded)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	h.ServeHTTP(w, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Less(t, elapsed, 250*time.Millisecond)

	var response models.ChatSummaryResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response.User)
	assert.NotNil(t, response.Permissions)
	assert.Nil(t, response.Context)
	assert.True(t, response.Degraded)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
}

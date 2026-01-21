package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vwency/resilient-scatter-gather/internal/handler"
	"github.com/vwency/resilient-scatter-gather/internal/models"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

func TestServeHTTP_PermissionsServiceError_ReturnsInternalServerError(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil)

	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(nil, errors.New("permissions service down"))

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{}}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, errResponse.Code)
	assert.Contains(t, errResponse.Message, "permissions")

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
}

func TestServeHTTP_PermissionsServiceError_DatabaseFailure(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{
		UserId:   "user123",
		Username: "testuser",
	}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(nil, errors.New("database query failed")).Run(func(args mock.Arguments) {
		time.Sleep(20 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "test"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

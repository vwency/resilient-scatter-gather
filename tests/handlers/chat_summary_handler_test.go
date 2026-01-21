package handler_test

import (
	"context"
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
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type MockUserServiceClient struct {
	mock.Mock
}

func (m *MockUserServiceClient) GetUser(ctx context.Context, userID string) (*pb_user.GetUserResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb_user.GetUserResponse), args.Error(1)
}

type MockPermissionsServiceClient struct {
	mock.Mock
}

func (m *MockPermissionsServiceClient) CheckAccess(ctx context.Context, userID, resourceID string) (*pb_permissions.CheckAccessResponse, error) {
	args := m.Called(ctx, userID, resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb_permissions.CheckAccessResponse), args.Error(1)
}

type MockVectorMemoryServiceClient struct {
	mock.Mock
}

func (m *MockVectorMemoryServiceClient) GetContext(ctx context.Context, chatID string) (*pb_vector.GetContextResponse, error) {
	args := m.Called(ctx, chatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb_vector.GetContextResponse), args.Error(1)
}

func TestServeHTTP_AllServicesSuccess(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{
		UserId:   "user123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{
		Allowed:     true,
		Permissions: []string{"chat:read", "chat:write"},
		Reason:      "User has access",
	}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "test context"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var response models.ChatSummaryResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response.User)
	assert.NotNil(t, response.Permissions)
	assert.NotNil(t, response.Context)
	assert.False(t, response.Degraded)
	assert.Equal(t, "user123", response.User.UserId)
	assert.True(t, response.Permissions.Allowed)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestServeHTTP_VectorServiceTimeout_GracefulDegradation(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123", Username: "testuser"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	mockVector.On("GetContext", mock.Anything, "chat1").Return(nil, context.DeadlineExceeded).Run(func(args mock.Arguments) {
		time.Sleep(300 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

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

func TestServeHTTP_UserServiceFailure_CriticalError(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	mockUser.On("GetUser", mock.Anything, "user123").Return(nil, errors.New("user service down"))

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil)

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

	mockUser.AssertExpectations(t)
}

func TestServeHTTP_PermissionsServiceFailure_CriticalError(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

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

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
}

func TestServeHTTP_UserServiceTimeout_CriticalError(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

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

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestServeHTTP_VectorServiceFailure_GracefulDegradation(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123", Username: "testuser"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true, Permissions: []string{"read"}}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	mockVector.On("GetContext", mock.Anything, "chat1").Return(nil, errors.New("vector service error"))

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.ChatSummaryResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response.User)
	assert.NotNil(t, response.Permissions)
	assert.Nil(t, response.Context)
	assert.True(t, response.Degraded)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestServeHTTP_MissingParameters(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	tests := []struct {
		name     string
		url      string
		expected int
	}{
		{
			name:     "missing user_id",
			url:      "/api/chat-summary?chat_id=chat1",
			expected: http.StatusBadRequest,
		},
		{
			name:     "missing chat_id",
			url:      "/api/chat-summary?user_id=user123",
			expected: http.StatusBadRequest,
		},
		{
			name:     "missing both parameters",
			url:      "/api/chat-summary",
			expected: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			assert.Equal(t, tt.expected, w.Code)

			var errResponse models.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&errResponse)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, errResponse.Code)
		})
	}
}

func TestServeHTTP_WithinSLA(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{{Content: "ctx"}}}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	h.ServeHTTP(w, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Less(t, elapsed, 250*time.Millisecond)

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

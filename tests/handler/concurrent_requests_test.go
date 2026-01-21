package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestServeHTTP_MultipleConcurrentRequests_AllSucceed(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123", Username: "testuser"}
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

	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	results := make([]int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			results[index] = w.Code
		}(i)
	}

	wg.Wait()

	for i, code := range results {
		assert.Equal(t, http.StatusOK, code, "Request %d failed", i)
	}
}

func TestServeHTTP_ConcurrentRequestsWithDegradation_HandleCorrectly(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, "user123").Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, "user123", "chat1").Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	mockVector.On("GetContext", mock.Anything, "chat1").Return(nil, errors.New("vector service down"))

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	numRequests := 5
	var wg sync.WaitGroup
	wg.Add(numRequests)

	degradedCount := 0
	var mu sync.Mutex

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var response models.ChatSummaryResponse
				json.NewDecoder(w.Body).Decode(&response)
				if response.Degraded {
					mu.Lock()
					degradedCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, numRequests, degradedCount)
}

func TestServeHTTP_ConcurrentRequestsMixedScenarios_HandleCorrectly(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, mock.Anything).Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, mock.Anything, mock.Anything).Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{{Content: "ctx"}}}
	mockVector.On("GetContext", mock.Anything, "chat1").Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})
	mockVector.On("GetContext", mock.Anything, "chat2").Return(nil, errors.New("error"))

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	results := make([]bool, 2)

	go func() {
		defer wg.Done()
		req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		var response models.ChatSummaryResponse
		json.NewDecoder(w.Body).Decode(&response)
		results[0] = !response.Degraded
	}()

	go func() {
		defer wg.Done()
		req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=chat2", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		var response models.ChatSummaryResponse
		json.NewDecoder(w.Body).Decode(&response)
		results[1] = response.Degraded
	}()

	wg.Wait()

	assert.True(t, results[0], "chat1 should succeed without degradation")
	assert.True(t, results[1], "chat2 should be degraded")
}

package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vwency/resilient-scatter-gather/internal/handler"
	"github.com/vwency/resilient-scatter-gather/internal/models"
)

func TestServeHTTP_MissingUserID_ReturnsBadRequest(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, errResponse.Code)
	assert.Contains(t, errResponse.Message, "user_id")
}

func TestServeHTTP_MissingChatID_ReturnsBadRequest(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, errResponse.Code)
	assert.Contains(t, errResponse.Message, "chat_id")
}

func TestServeHTTP_MissingBothParameters_ReturnsBadRequest(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, errResponse.Code)
}

func TestServeHTTP_EmptyUserID_ReturnsBadRequest(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, errResponse.Code)
}

func TestServeHTTP_EmptyChatID_ReturnsBadRequest(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("GET", "/api/chat-summary?user_id=user123&chat_id=", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResponse models.ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&errResponse)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, errResponse.Code)
}

func TestServeHTTP_InvalidMethod_ReturnsMethodNotAllowed(t *testing.T) {
	mockUser := new(UserServiceClient)
	mockPermissions := new(PermissionsServiceClient)
	mockVector := new(VectorMemoryServiceClient)

	h := handler.NewChatSummaryHandler(mockUser, mockVector, mockPermissions, 200*time.Millisecond)

	req := httptest.NewRequest("POST", "/api/chat-summary?user_id=user123&chat_id=chat1", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

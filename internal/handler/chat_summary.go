package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/vwency/resilient-scatter-gather/internal/models"
	"github.com/vwency/resilient-scatter-gather/internal/services"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

const (
	GlobalSLA = 200 * time.Millisecond
)

type ChatSummaryHandler struct {
	userService        *services.UserServiceClient
	vectorService      *services.VectorMemoryServiceClient
	permissionsService *services.PermissionsServiceClient
}

func NewChatSummaryHandler(
	userService *services.UserServiceClient,
	vectorService *services.VectorMemoryServiceClient,
	permissionsService *services.PermissionsServiceClient,
) *ChatSummaryHandler {
	return &ChatSummaryHandler{
		userService:        userService,
		vectorService:      vectorService,
		permissionsService: permissionsService,
	}
}

type serviceResult struct {
	userData        *pb_user.GetUserResponse
	permissionsData *pb_permissions.CheckAccessResponse
	contextData     *pb_vector.GetContextResponse
	err             error
	serviceName     string
}

func (h *ChatSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), GlobalSLA)
	defer cancel()

	userID := r.URL.Query().Get("user_id")
	chatID := r.URL.Query().Get("chat_id")

	if userID == "" || chatID == "" {
		h.sendError(w, "user_id and chat_id are required", http.StatusBadRequest)
		return
	}

	start := time.Now()
	userData, permissionsData, contextData, degraded, err := h.scatterGather(ctx, userID, chatID)
	elapsed := time.Since(start)

	log.Printf("Request completed in %v (degraded: %v)", elapsed, degraded)

	if err != nil {
		log.Printf("Critical service failure: %v", err)
		h.sendError(w, fmt.Sprintf("Service unavailable: %v", err), http.StatusInternalServerError)
		return
	}

	response := &models.ChatSummaryResponse{
		User:        userData,
		Permissions: permissionsData,
		Context:     contextData,
		Degraded:    degraded,
		Timestamp:   time.Now(),
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *ChatSummaryHandler) scatterGather(ctx context.Context, userID, chatID string) (
	*pb_user.GetUserResponse,
	*pb_permissions.CheckAccessResponse,
	*pb_vector.GetContextResponse,
	bool,
	error,
) {
	var wg sync.WaitGroup
	results := make(chan serviceResult, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		user, err := h.userService.GetUser(ctx, userID)
		results <- serviceResult{
			userData:    user,
			err:         err,
			serviceName: "UserService",
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		perms, err := h.permissionsService.CheckAccess(ctx, userID, chatID)
		results <- serviceResult{
			permissionsData: perms,
			err:             err,
			serviceName:     "PermissionsService",
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		contextData, err := h.vectorService.GetContext(ctx, chatID)
		results <- serviceResult{
			contextData: contextData,
			err:         err,
			serviceName: "VectorMemoryService",
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var (
		userData        *pb_user.GetUserResponse
		permissionsData *pb_permissions.CheckAccessResponse
		contextData     *pb_vector.GetContextResponse
		degraded        bool
	)

	for result := range results {
		switch result.serviceName {
		case "UserService":
			if result.err != nil {
				return nil, nil, nil, false, fmt.Errorf("user service failed: %w", result.err)
			}
			userData = result.userData
			log.Printf("✓ UserService succeeded")

		case "PermissionsService":
			if result.err != nil {
				return nil, nil, nil, false, fmt.Errorf("permissions service failed: %w", result.err)
			}
			permissionsData = result.permissionsData
			log.Printf("✓ PermissionsService succeeded")

		case "VectorMemoryService":
			if result.err != nil {
				log.Printf("⚠ VectorMemoryService failed (degraded): %v", result.err)
				degraded = true
				contextData = nil
			} else {
				contextData = result.contextData
				log.Printf("✓ VectorMemoryService succeeded")
			}
		}
	}

	return userData, permissionsData, contextData, degraded, nil
}

func (h *ChatSummaryHandler) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

func (h *ChatSummaryHandler) sendError(w http.ResponseWriter, message string, statusCode int) {
	errResp := &models.ErrorResponse{
		Error:   http.StatusText(statusCode),
		Code:    statusCode,
		Message: message,
	}
	h.sendJSON(w, errResp, statusCode)
}

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/vwency/resilient-scatter-gather/internal/models"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type UserServiceClient interface {
	GetUser(ctx context.Context, userID string) (*pb_user.GetUserResponse, error)
}

type PermissionsServiceClient interface {
	CheckAccess(ctx context.Context, userID, resourceID string) (*pb_permissions.CheckAccessResponse, error)
}

type VectorMemoryServiceClient interface {
	GetContext(ctx context.Context, chatID string) (*pb_vector.GetContextResponse, error)
}

type ChatSummaryHandler struct {
	userService        UserServiceClient
	vectorService      VectorMemoryServiceClient
	permissionsService PermissionsServiceClient
	slaTimeout         time.Duration
}

func NewChatSummaryHandler(
	userService UserServiceClient,
	vectorService VectorMemoryServiceClient,
	permissionsService PermissionsServiceClient,
	slaTimeout time.Duration,
) *ChatSummaryHandler {
	return &ChatSummaryHandler{
		userService:        userService,
		vectorService:      vectorService,
		permissionsService: permissionsService,
		slaTimeout:         slaTimeout,
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
	ctx, cancel := context.WithTimeout(r.Context(), h.slaTimeout)
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
	results := make(chan serviceResult, 3)

	go func() {
		user, err := h.userService.GetUser(ctx, userID)
		results <- serviceResult{
			userData:    user,
			err:         err,
			serviceName: "UserService",
		}
	}()

	go func() {
		perms, err := h.permissionsService.CheckAccess(ctx, userID, chatID)
		results <- serviceResult{
			permissionsData: perms,
			err:             err,
			serviceName:     "PermissionsService",
		}
	}()

	go func() {
		contextData, err := h.vectorService.GetContext(ctx, chatID)
		results <- serviceResult{
			contextData: contextData,
			err:         err,
			serviceName: "VectorMemoryService",
		}
	}()

	var (
		userData        *pb_user.GetUserResponse
		permissionsData *pb_permissions.CheckAccessResponse
		contextData     *pb_vector.GetContextResponse
		degraded        bool
		received        int
	)

	for received < 3 {
		select {
		case result := <-results:
			received++
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

		case <-ctx.Done():
			log.Printf("⚠ Context timeout reached, stopping collection")
			if userData == nil || permissionsData == nil {
				return nil, nil, nil, false, fmt.Errorf("critical services timeout")
			}
			degraded = true
			return userData, permissionsData, contextData, degraded, nil
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

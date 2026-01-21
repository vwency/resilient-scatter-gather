package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/vwency/resilient-scatter-gather/internal/models"
	"github.com/vwency/resilient-scatter-gather/internal/services"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type ChatSummaryHandler struct {
	userService        services.UserService
	vectorService      services.VectorMemoryService
	permissionsService services.PermissionsService
	slaTimeout         time.Duration
}

func NewChatSummaryHandler(
	userService services.UserService,
	vectorService services.VectorMemoryService,
	permissionsService services.PermissionsService,
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

func (h *ChatSummaryHandler) Handle(ctx *fasthttp.RequestCtx) {
	reqCtx, cancel := context.WithTimeout(context.Background(), h.slaTimeout)
	defer cancel()

	userID := string(ctx.QueryArgs().Peek("user_id"))
	chatID := string(ctx.QueryArgs().Peek("chat_id"))

	if userID == "" || chatID == "" {
		h.sendError(ctx, "user_id and chat_id are required", fasthttp.StatusBadRequest)
		return
	}

	start := time.Now()
	userData, permissionsData, contextData, degraded, err := h.scatterGather(reqCtx, userID, chatID)
	elapsed := time.Since(start)

	log.Printf("Request completed in %v (degraded: %v)", elapsed, degraded)

	if err != nil {
		log.Printf("Critical service failure: %v", err)
		h.sendError(ctx, fmt.Sprintf("Service unavailable: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	response := &models.ChatSummaryResponse{
		User:        userData,
		Permissions: permissionsData,
		Context:     contextData,
		Degraded:    degraded,
		Timestamp:   time.Now(),
	}

	h.sendJSON(ctx, response, fasthttp.StatusOK)
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

func (h *ChatSummaryHandler) sendJSON(ctx *fasthttp.RequestCtx, data interface{}, statusCode int) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(statusCode)

	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error encoding JSON: %v", err)
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	ctx.Response.SetBody(body)
}

func (h *ChatSummaryHandler) sendError(ctx *fasthttp.RequestCtx, message string, statusCode int) {
	errResp := &models.ErrorResponse{
		Error:   fasthttp.StatusMessage(statusCode),
		Code:    statusCode,
		Message: message,
	}
	h.sendJSON(ctx, errResp, statusCode)
}

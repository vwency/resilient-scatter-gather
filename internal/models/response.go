package models

import (
	"time"

	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type ChatSummaryResponse struct {
	User        *pb_user.GetUserResponse            `json:"user"`
	Permissions *pb_permissions.CheckAccessResponse `json:"permissions"`
	Context     *pb_vector.GetContextResponse       `json:"context,omitempty"`
	Degraded    bool                                `json:"degraded"`
	Timestamp   time.Time                           `json:"timestamp"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

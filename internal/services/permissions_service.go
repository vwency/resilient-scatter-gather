package services

import (
	"context"
	"time"

	pb "github.com/vwency/resilient-scatter-gather/proto/permissions"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PermissionsServiceClient struct {
	client             pb.PermissionsServiceClient
	degradationTimeout time.Duration
}

func NewPermissionsServiceClient(client pb.PermissionsServiceClient, degradationTimeout time.Duration) *PermissionsServiceClient {
	return &PermissionsServiceClient{
		client:             client,
		degradationTimeout: degradationTimeout,
	}
}

func (s *PermissionsServiceClient) CheckAccess(ctx context.Context, userID, resourceID string) (*pb.CheckAccessResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.degradationTimeout)
	defer cancel()

	req := &pb.CheckAccessRequest{
		UserId:     userID,
		ResourceId: resourceID,
		Action:     "read",
	}

	resp, err := s.client.CheckAccess(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, status.Error(codes.Internal, "Permissions service timeout")
		}
		return nil, err
	}

	return resp, nil
}

type PermissionsServiceServer struct {
	pb.UnimplementedPermissionsServiceServer
}

func NewPermissionsServiceServer() *PermissionsServiceServer {
	return &PermissionsServiceServer{}
}

func (s *PermissionsServiceServer) CheckAccess(ctx context.Context, req *pb.CheckAccessRequest) (*pb.CheckAccessResponse, error) {
	return &pb.CheckAccessResponse{
		Allowed: true,
		Permissions: []string{
			"chat:read",
			"chat:write",
			"chat:summary:view",
		},
		Reason: "User has full access to the chat",
	}, nil
}

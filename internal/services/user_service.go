package services

import (
	"context"
	"time"

	pb "github.com/vwency/resilient-scatter-gather/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserServiceClient struct {
	client             pb.UserServiceClient
	degradationTimeout time.Duration
}

func NewUserServiceClient(client pb.UserServiceClient, degradationTimeout time.Duration) *UserServiceClient {
	return &UserServiceClient{
		client:             client,
		degradationTimeout: degradationTimeout,
	}
}

func (s *UserServiceClient) GetUser(ctx context.Context, userID string) (*pb.GetUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.degradationTimeout)
	defer cancel()

	req := &pb.GetUserRequest{UserId: userID}
	resp, err := s.client.GetUser(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, status.Error(codes.Internal, "User service timeout")
		}
		return nil, err
	}

	return resp, nil
}

type UserServiceServer struct {
	pb.UnimplementedUserServiceServer
}

func NewUserServiceServer() *UserServiceServer {
	return &UserServiceServer{}
}

func (s *UserServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		UserId:   req.UserId,
		Username: req.UserId,
		Email:    "",
		Role:     "",
	}, nil
}

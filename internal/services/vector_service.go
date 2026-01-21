package services

import (
	"context"
	"time"

	pb "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type VectorMemoryServiceClient struct {
	client             pb.VectorMemoryServiceClient
	degradationTimeout time.Duration
}

func NewVectorMemoryServiceClient(client pb.VectorMemoryServiceClient, degradationTimeout time.Duration) *VectorMemoryServiceClient {
	return &VectorMemoryServiceClient{
		client:             client,
		degradationTimeout: degradationTimeout,
	}
}

func (s *VectorMemoryServiceClient) GetContext(ctx context.Context, chatID string) (*pb.GetContextResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, s.degradationTimeout)
	defer cancel()

	req := &pb.GetContextRequest{
		ChatId: chatID,
		Limit:  10,
	}

	resp, err := s.client.GetContext(ctx, req)
	if err != nil {
		return &pb.GetContextResponse{
			Items:      []*pb.ContextItem{},
			TotalCount: 0,
		}, nil
	}

	return resp, nil
}

type VectorMemoryServiceServer struct {
	pb.UnimplementedVectorMemoryServiceServer
}

func NewVectorMemoryServiceServer() *VectorMemoryServiceServer {
	return &VectorMemoryServiceServer{}
}

func (s *VectorMemoryServiceServer) GetContext(ctx context.Context, req *pb.GetContextRequest) (*pb.GetContextResponse, error) {
	return &pb.GetContextResponse{
		Items:      []*pb.ContextItem{},
		TotalCount: 0,
	}, nil
}

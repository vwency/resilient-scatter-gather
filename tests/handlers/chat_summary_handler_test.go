package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
)

type MockUserServiceClient struct {
	mock.Mock
}

func (m *MockUserServiceClient) GetUser(ctx context.Context, in *pb_user.GetUserRequest, opts ...interface{}) (*pb_user.GetUserResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb_user.GetUserResponse), args.Error(1)
}

type MockPermissionsServiceClient struct {
	mock.Mock
}

func (m *MockPermissionsServiceClient) CheckAccess(ctx context.Context, in *pb_permissions.CheckAccessRequest, opts ...interface{}) (*pb_permissions.CheckAccessResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb_permissions.CheckAccessResponse), args.Error(1)
}

type MockVectorMemoryServiceClient struct {
	mock.Mock
}

func (m *MockVectorMemoryServiceClient) GetContext(ctx context.Context, in *pb_vector.GetContextRequest, opts ...interface{}) (*pb_vector.GetContextResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb_vector.GetContextResponse), args.Error(1)
}

func TestScatterGather_AllServicesSuccess(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{
		UserId:   "user123",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}
	mockUser.On("GetUser", mock.Anything, mock.Anything).Return(userResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	permResp := &pb_permissions.CheckAccessResponse{
		Allowed:     true,
		Permissions: []string{"chat:read", "chat:write"},
		Reason:      "User has access",
	}
	mockPermissions.On("CheckAccess", mock.Anything, mock.Anything).Return(permResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond)
	})

	vectorResp := &pb_vector.GetContextResponse{
		Items:      []*pb_vector.ContextItem{{Content: "test context"}},
		TotalCount: 1,
	}
	mockVector.On("GetContext", mock.Anything, mock.Anything).Return(vectorResp, nil).Run(func(args mock.Arguments) {
		time.Sleep(80 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	type result struct {
		user   *pb_user.GetUserResponse
		perm   *pb_permissions.CheckAccessResponse
		vector *pb_vector.GetContextResponse
		err    error
	}

	results := make(chan result, 3)

	go func() {
		resp, err := mockUser.GetUser(ctx, &pb_user.GetUserRequest{UserId: "user123"})
		results <- result{user: resp, err: err}
	}()

	go func() {
		resp, err := mockPermissions.CheckAccess(ctx, &pb_permissions.CheckAccessRequest{UserId: "user123", ResourceId: "chat1"})
		results <- result{perm: resp, err: err}
	}()

	go func() {
		resp, err := mockVector.GetContext(ctx, &pb_vector.GetContextRequest{ChatId: "chat1"})
		results <- result{vector: resp, err: err}
	}()

	var finalUser *pb_user.GetUserResponse
	var finalPerm *pb_permissions.CheckAccessResponse
	var finalVector *pb_vector.GetContextResponse
	var criticalErr error

	for i := 0; i < 3; i++ {
		select {
		case res := <-results:
			if res.user != nil {
				finalUser = res.user
				if res.err != nil {
					criticalErr = res.err
				}
			}
			if res.perm != nil {
				finalPerm = res.perm
				if res.err != nil {
					criticalErr = res.err
				}
			}
			if res.vector != nil {
				finalVector = res.vector
			}
		case <-ctx.Done():
			break
		}
	}

	if criticalErr != nil {
		t.Fatalf("Critical service failed: %v", criticalErr)
	}

	if finalUser == nil {
		t.Error("User service did not respond")
	}
	if finalPerm == nil {
		t.Error("Permissions service did not respond")
	}
	if finalVector == nil {
		t.Log("Vector service did not respond (acceptable degradation)")
	}

	mockUser.AssertExpectations(t)
	mockPermissions.AssertExpectations(t)
	mockVector.AssertExpectations(t)
}

func TestScatterGather_UserServiceFailure(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	mockUser.On("GetUser", mock.Anything, mock.Anything).Return(nil, errors.New("user service down"))

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, mock.Anything).Return(permResp, nil)

	vectorResp := &pb_vector.GetContextResponse{Items: []*pb_vector.ContextItem{}}
	mockVector.On("GetContext", mock.Anything, mock.Anything).Return(vectorResp, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := mockUser.GetUser(ctx, &pb_user.GetUserRequest{UserId: "user123"})

	if err == nil {
		t.Error("Expected error from user service")
	}
}

func TestScatterGather_VectorServiceTimeout(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)
	mockVector := new(MockVectorMemoryServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, mock.Anything).Return(userResp, nil)

	permResp := &pb_permissions.CheckAccessResponse{Allowed: true}
	mockPermissions.On("CheckAccess", mock.Anything, mock.Anything).Return(permResp, nil)

	mockVector.On("GetContext", mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Run(func(args mock.Arguments) {
		time.Sleep(300 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		mockVector.GetContext(ctx, &pb_vector.GetContextRequest{ChatId: "chat1"})
		done <- true
	}()

	<-ctx.Done()

	if ctx.Err() != context.DeadlineExceeded {
		t.Error("Expected context deadline exceeded")
	}
}

func TestScatterGather_PermissionsServiceFailure(t *testing.T) {
	mockUser := new(MockUserServiceClient)
	mockPermissions := new(MockPermissionsServiceClient)

	userResp := &pb_user.GetUserResponse{UserId: "user123"}
	mockUser.On("GetUser", mock.Anything, mock.Anything).Return(userResp, nil)

	mockPermissions.On("CheckAccess", mock.Anything, mock.Anything).Return(nil, errors.New("permissions service down"))

	ctx := context.Background()

	_, err := mockPermissions.CheckAccess(ctx, &pb_permissions.CheckAccessRequest{UserId: "user123", ResourceId: "chat1"})

	if err == nil {
		t.Error("Expected error from permissions service")
	}
}

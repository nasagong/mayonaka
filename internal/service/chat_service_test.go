package service

import (
	"context"
	"testing"

	"github.com/nasagong/mayonaka/internal/errors"
	status "github.com/nasagong/mayonaka/internal/status"
)

type MockChatRepository struct {
	roomActive bool
}

func (m *MockChatRepository) CheckRoomActive(ctx context.Context, roomId string) (status.RoomStatus, error) {
	if m.roomActive {
		return status.RoomActive, nil
	}
	return status.RoomNotFound, nil
}

func (m *MockChatRepository) FindMaster(ctx context.Context, roomId string) (string, error) {
	if m.roomActive {
		return "user1", nil
	}
	return "", errors.ErrRoomNotFound
}

func (m *MockChatRepository) CheckDuplicateUser(ctx context.Context, username string, roomId string) (bool, error) {
	return false, nil
}

func (m *MockChatRepository) CreateRoom(ctx context.Context, username string, roomId string) (bool, error) {
	return true, nil
}

func (m *MockChatRepository) DeleteUserFromRoom(ctx context.Context, username string, roomId string) (bool, error) {
	return true, nil
}

func (m *MockChatRepository) AddUserToRoom(ctx context.Context, username string, roomId string) (bool, error) {
	return true, nil
}

func (m *MockChatRepository) StorePublicKey(ctx context.Context, username string, roomId string, publicKey string) (bool, error) {
	return true, nil
}

func (m *MockChatRepository) StoreEncryptedAESKey(ctx context.Context, roomId string, encryptedAESKey string) error {
	return nil
}

func (m *MockChatRepository) GetEncryptedAESKey(ctx context.Context, roomId string) (string, error) {
	return "mock-aes-key", nil
}

func (m *MockChatRepository) GetPublicKey(ctx context.Context, username string, roomId string) (string, error) {
	return "mock-public-key", nil
}

func (m *MockChatRepository) DeleteRoomData(ctx context.Context, roomId string) error {
	return nil
}

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name        string
		roomActive  bool
		wantSuccess bool
		wantErr     error
	}{
		{"ActiveRoom", true, true, nil},
		{"RoomNotFound", false, false, errors.ErrRoomNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockChatRepository{roomActive: tt.roomActive}
			s := NewChatService(repo)
			res, err := s.SendMessage(context.Background(), "room1", "user1", "Hello")

			if res.Success != tt.wantSuccess {
				t.Errorf("SendMessage() success = %v, want %v", res.Success, tt.wantSuccess)
			}
			if err != tt.wantErr {
				t.Errorf("SendMessage() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindMaster(t *testing.T) {
	tests := []struct {
		name       string
		roomActive bool
		wantMaster string
		wantErr    bool
	}{
		{"MasterExists", true, "user1", false},
		{"NoMaster", false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockChatRepository{roomActive: tt.roomActive}
			s := NewChatService(repo)
			master, err := s.FindMaster(context.Background(), "room1")

			if master != tt.wantMaster {
				t.Errorf("FindMaster() master = %v, want %v", master, tt.wantMaster)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("FindMaster() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

package service

import (
	"context"
	"fmt"

	"github.com/nasagong/mayonaka/internal/errors"
	pb "github.com/nasagong/mayonaka/internal/pb/chat"
	"github.com/nasagong/mayonaka/internal/repository"
	status "github.com/nasagong/mayonaka/internal/status"
)

type ChatService interface {
	SendMessage(ctx context.Context, roomId string, sender string, content string) (*pb.SendResponse, error)
	CheckDuplicateUser(ctx context.Context, roomId string, username string) (*pb.CheckUserNameResponse, error)
	CreateRoom(ctx context.Context, username string, roomId string, pubKey string, encryptedAESKey string) (*pb.CreateRoomResponse, error)
	QuitRoom(ctx context.Context, username string, roomId string) (*pb.QuitResponse, error)
	FindMaster(ctx context.Context, roomId string) (string, error)
	StoreEncryptedAESKey(ctx context.Context, roomId string, encryptedAESKey string) error
}

type chatService struct {
	repo repository.ChatRepository
}

func NewChatService(repo repository.ChatRepository) ChatService {
	return &chatService{repo: repo}
}

func (s *chatService) newResponse(code pb.RoomCode, roomId, aesKey string) *pb.CreateRoomResponse {
	return &pb.CreateRoomResponse{Code: code, RoomId: roomId, AesKey: aesKey}
}

func (s *chatService) CheckDuplicateUser(ctx context.Context, roomId string, username string) (*pb.CheckUserNameResponse, error) {
	duplicated, err := s.repo.CheckDuplicateUser(ctx, roomId, username)
	if err != nil {
		return &pb.CheckUserNameResponse{IsDuplicated: false}, errors.ErrRedisConnection
	}
	return &pb.CheckUserNameResponse{IsDuplicated: duplicated}, nil
}

func (s *chatService) SendMessage(ctx context.Context, roomId string, sender string, content string) (*pb.SendResponse, error) {
	active, err := s.repo.CheckRoomActive(ctx, roomId)
	if err != nil {
		return &pb.SendResponse{Success: false}, errors.ErrUnknown
	}
	if active == status.RoomNotFound {
		return &pb.SendResponse{Success: false}, errors.ErrRoomNotFound
	}
	return &pb.SendResponse{Success: true}, nil
}

func (s *chatService) handleRoom(ctx context.Context, roomId, username, pubKey string, mainOp func() (bool, error)) (*pb.CreateRoomResponse, error) {
	success, err := mainOp()
	if err != nil || !success {
		return s.newResponse(pb.RoomCode_UNKNOWN_ERROR, roomId, ""), errors.ErrUnknown
	}
	stored, err := s.repo.StorePublicKey(ctx, username, roomId, pubKey)
	if err != nil || !stored {
		return s.newResponse(pb.RoomCode_UNKNOWN_ERROR, roomId, ""), errors.ErrUnknown
	}
	return nil, nil
}

func (s *chatService) CreateRoom(ctx context.Context, username string, roomId string, pubKey string, encryptedAESKey string) (*pb.CreateRoomResponse, error) {
	roomStatus, err := s.repo.CheckRoomActive(ctx, roomId)
	if err != nil {
		return s.newResponse(pb.RoomCode_UNKNOWN_ERROR, roomId, ""), err
	}

	switch roomStatus {
	case status.RoomActive:
		result, err := s.handleRoom(ctx, roomId, username, pubKey, func() (bool, error) {
			return s.repo.AddUserToRoom(ctx, username, roomId)
		})
		if result != nil {
			return result, err
		}
		return s.newResponse(pb.RoomCode_ID_ALREADY_EXISTS, roomId, ""), nil

	case status.RoomNotFound:
		result, err := s.handleRoom(ctx, roomId, username, pubKey, func() (bool, error) {
			return s.repo.CreateRoom(ctx, username, roomId)
		})
		if result != nil {
			return result, err
		}
		if encryptedAESKey != "" {
			if err := s.repo.StoreEncryptedAESKey(ctx, roomId, encryptedAESKey); err != nil {
				return s.newResponse(pb.RoomCode_UNKNOWN_ERROR, roomId, ""), err
			}
		}
		return s.newResponse(pb.RoomCode_SUCCESS, roomId, ""), nil

	default:
		return s.newResponse(pb.RoomCode_UNKNOWN_ERROR, roomId, ""), fmt.Errorf("unknown room status")
	}
}

func (s *chatService) QuitRoom(ctx context.Context, username string, roomId string) (*pb.QuitResponse, error) {
	result, err := s.repo.DeleteUserFromRoom(ctx, username, roomId)
	if err != nil {
		return &pb.QuitResponse{Success: result}, err
	}
	return &pb.QuitResponse{Success: result}, nil
}

func (s *chatService) FindMaster(ctx context.Context, roomId string) (string, error) {
	return s.repo.FindMaster(ctx, roomId)
}

func (s *chatService) StoreEncryptedAESKey(ctx context.Context, roomId, encryptedAESKey string) error {
	return s.repo.StoreEncryptedAESKey(ctx, roomId, encryptedAESKey)
}

package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	status "github.com/nasagong/mayonaka/internal/status"
	"github.com/redis/go-redis/v9"
)

type RoomStatus int

const (
	RoomActivate RoomStatus = iota
	RoomNotFound
	RoomUnknownError
)

type ChatRepository interface {
	CheckRoomActive(ctx context.Context, roomId string) (status.RoomStatus, error)
	CheckDuplicateUser(ctx context.Context, username string, roomId string) (bool, error)
	CreateRoom(ctx context.Context, username string, roomId string) (bool, error)
	DeleteUserFromRoom(ctx context.Context, username string, roomId string) (bool, error)
	AddUserToRoom(ctx context.Context, username string, roomId string) (bool, error)
	StorePublicKey(ctx context.Context, username string, roomId string, publicKey string) (bool, error)
	FindMaster(ctx context.Context, roomId string) (string, error)
	StoreEncryptedAESKey(ctx context.Context, roomId string, encryptedAESKey string) error
	GetEncryptedAESKey(ctx context.Context, roomId string) (string, error)
	GetPublicKey(ctx context.Context, username string, roomId string) (string, error)
	DeleteRoomData(ctx context.Context, roomId string) error
}

type chatRepository struct {
	redisClient *redis.Client
}

func NewChatRepository(redisClient *redis.Client) ChatRepository {
	return &chatRepository{redisClient: redisClient}
}

func (r *chatRepository) roomKey(roomId string) string {
	return "room:" + roomId
}

func (r *chatRepository) keysKey(roomId string) string {
	return fmt.Sprintf("room_keys:%s", roomId)
}

func (r *chatRepository) aesKey(roomId string) string {
	return fmt.Sprintf("room_aes:%s", roomId)
}

func (r *chatRepository) logError(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

func (r *chatRepository) CheckDuplicateUser(ctx context.Context, roomId string, username string) (bool, error) {
	key := r.roomKey(roomId)
	_, err := r.redisClient.ZScore(ctx, key, username).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		r.logError("Error checking user duplication for room %s, user %s: %v", roomId, username, err)
		return false, err
	}
	return true, nil
}

func (r *chatRepository) CheckRoomActive(ctx context.Context, roomId string) (status.RoomStatus, error) {
	score, err := r.redisClient.ZScore(ctx, "active_rooms", roomId).Result()
	if err == redis.Nil {
		return status.RoomNotFound, nil
	}
	if err != nil {
		return status.RoomUnknownError, err
	}
	if score > float64(time.Now().Unix()) {
		return status.RoomActive, nil
	}
	r.redisClient.Del(ctx, r.roomKey(roomId))
	r.redisClient.ZRem(ctx, "active_rooms", roomId)
	return status.RoomNotFound, nil
}

func (r *chatRepository) CreateRoom(ctx context.Context, username string, roomId string) (bool, error) {
	expiration := float64(time.Now().Add(1 * time.Hour).Unix())
	result, err := r.redisClient.ZAdd(ctx, "active_rooms", redis.Z{
		Score:  expiration,
		Member: roomId,
	}).Result()
	if err != nil || result == 0 {
		return false, err
	}
	return r.AddUserToRoom(ctx, username, roomId)
}

func (r *chatRepository) AddUserToRoom(ctx context.Context, username string, roomId string) (bool, error) {
	timestamp := float64(time.Now().Unix())
	result, err := r.redisClient.ZAdd(ctx, r.roomKey(roomId), redis.Z{
		Score:  timestamp,
		Member: username,
	}).Result()
	log.Printf("ADD %s to %s", username, roomId)
	return result == 1, err
}

func (r *chatRepository) DeleteUserFromRoom(ctx context.Context, username string, roomId string) (bool, error) {
	result, err := r.redisClient.ZRem(ctx, r.roomKey(roomId), username).Result()
	if err != nil {
		return false, err
	}
	if result > 0 {
		count, err := r.redisClient.ZCard(ctx, r.roomKey(roomId)).Result()
		if err != nil {
			r.logError("Failed to get member count for room %s: %v", roomId, err)
			return true, nil
		}
		if count == 0 {
			if err := r.DeleteRoomData(ctx, roomId); err != nil {
				r.logError("Failed to delete room data for %s: %v", roomId, err)
			}
			r.redisClient.ZRem(ctx, "active_rooms", roomId)
		}
	}
	return result > 0, nil
}

func (r *chatRepository) FindMaster(ctx context.Context, roomId string) (string, error) {
	members, err := r.redisClient.ZRange(ctx, r.roomKey(roomId), 0, 0).Result()
	if err != nil || len(members) == 0 {
		return "", fmt.Errorf("no master found for room %s", roomId)
	}
	return members[0], nil
}

func (r *chatRepository) StorePublicKey(ctx context.Context, username string, roomId string, publicKey string) (bool, error) {
	key := r.keysKey(roomId)
	err := r.redisClient.HSet(ctx, key, username, publicKey).Err()
	if err != nil {
		return false, fmt.Errorf("failed to store public key: %v", err)
	}
	return true, nil
}

func (r *chatRepository) StoreEncryptedAESKey(ctx context.Context, roomId string, encryptedAESKey string) error {
	key := r.aesKey(roomId)
	return r.redisClient.Set(ctx, key, encryptedAESKey, 1*time.Hour).Err()
}

func (r *chatRepository) GetEncryptedAESKey(ctx context.Context, roomId string) (string, error) {
	key := r.aesKey(roomId)
	val, err := r.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("AES key not found for room %s", roomId)
	}
	return val, err
}

func (r *chatRepository) GetPublicKey(ctx context.Context, username string, roomId string) (string, error) {
	key := r.keysKey(roomId)
	val, err := r.redisClient.HGet(ctx, key, username).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("public key not found for user %s in room %s", username, roomId)
	}
	return val, err
}

func (r *chatRepository) DeleteRoomData(ctx context.Context, roomId string) error {
	keys := []string{
		r.roomKey(roomId),
		r.keysKey(roomId),
		r.aesKey(roomId),
	}
	if err := r.redisClient.Del(ctx, keys...).Err(); err != nil {
		r.logError("Failed to delete Redis data for room %s: %v", roomId, err)
		return err
	}
	if err := r.redisClient.ZRem(ctx, "active_rooms", roomId).Err(); err != nil {
		r.logError("Failed to remove room %s from active_rooms: %v", roomId, err)
		return err
	}
	log.Printf("Cleaned up all Redis data for room %s", roomId)
	return nil
}

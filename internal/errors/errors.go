package errors

import "fmt"

type ChatError struct {
	Code    string
	Message string
}

func (e *ChatError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
	ErrRoomNotFound    = &ChatError{Code: "ROOM_NOT_FOUND", Message: "room does not exist"}
	ErrUserNotInRoom   = &ChatError{Code: "USER_NOT_IN_ROOM", Message: "user not in room"}
	ErrUnknown         = &ChatError{Code: "UNKNOWN_ERROR", Message: "unknown error occurred"}
	ErrRedisConnection = &ChatError{Code: "REDIS_ERROR", Message: "redis connection error"}
)

//go:build wireinject
// +build wireinject

package inject

import (
	"os"

	"github.com/google/wire"
	"github.com/nasagong/mayonaka/internal/handler"
	"github.com/nasagong/mayonaka/internal/repository"
	"github.com/nasagong/mayonaka/internal/service"
	"github.com/redis/go-redis/v9"
)

func InitializeChatHandler() *handler.ChatHandler {
	wire.Build(
		handler.NewChatHandler,
		service.NewChatService,
		repository.NewChatRepository,
		provideChatRedisClient,
	)
	return &handler.ChatHandler{}
}

func provideChatRedisClient() *redis.Client {
	redisAddr := os.Getenv("REDIS_ADDR")
	return redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
}

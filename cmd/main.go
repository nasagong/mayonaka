package main

import (
	"log"
	"net"

	"github.com/nasagong/mayonaka/internal/inject"
	pbChat "github.com/nasagong/mayonaka/internal/pb/chat"

	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to create tcp listener: %v", err)
	}

	grpcServer := grpc.NewServer()
	chatServer := inject.InitializeChatHandler()

	pbChat.RegisterChatServer(grpcServer, chatServer)

	log.Println("[Mayonaka] listening on 50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to launch grpc server : %v", err)
	}
}

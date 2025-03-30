package handler

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nasagong/mayonaka/internal/errors"
	pb "github.com/nasagong/mayonaka/internal/pb/chat"
	"github.com/nasagong/mayonaka/internal/service"
)

type ChatHandler struct {
	pb.UnimplementedChatServer
	chatService service.ChatService
	streams     map[string]map[string]chan *pb.ChatMessage
	pubKey      *rsa.PublicKey
	mu          sync.Mutex
}

func NewChatHandler(chatService service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		streams:     make(map[string]map[string]chan *pb.ChatMessage),
		pubKey:      nil,
	}
}

func (c *ChatHandler) withLock(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn()
}

func (c *ChatHandler) logError(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

func (c *ChatHandler) logInfo(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

func (c *ChatHandler) JoinOrCreateRoom(ctx context.Context, req *pb.CreateRoomRequest) (*pb.CreateRoomResponse, error) {
	var res *pb.CreateRoomResponse
	c.withLock(func() {
		var err error
		res, err = c.chatService.CreateRoom(ctx, req.Username, req.RoomId, req.PubKey, req.EncryptedAesKey)
		if err != nil {
			c.logError("Failed to create room %s for %s: %v", req.RoomId, req.Username, err)
			return
		}

		if _, exists := c.streams[req.RoomId]; !exists {
			c.streams[req.RoomId] = make(map[string]chan *pb.ChatMessage)
		}
		if _, exists := c.streams[req.RoomId][req.Username]; !exists {
			c.streams[req.RoomId][req.Username] = make(chan *pb.ChatMessage, 10)
		}

		if res.Code == pb.RoomCode_ID_ALREADY_EXISTS {
			master, err := c.chatService.FindMaster(ctx, req.RoomId)
			if err != nil {
				c.logError("Failed to find master for room %s: %v", req.RoomId, err)
				return
			}
			if master != req.Username {
				go c.notifyMaster(req.RoomId, master, req.PubKey)
			}
		}
	})
	return res, nil
}

// notifyMaster sends a new user's public key to the room master for secure key distribution
func (c *ChatHandler) notifyMaster(roomId, master, pubKey string) {
	c.withLock(func() {
		if roomStreams, exists := c.streams[roomId]; exists {
			if ch, ok := roomStreams[master]; ok {
				ch <- &pb.ChatMessage{
					RoomId:    roomId,
					Sender:    "system",
					Content:   pubKey,
					Type:      pb.MessageType_NEW_USER_PUBKEY,
					Timestamp: time.Now().Unix(),
				}
			}
		}
	})
}

// broadcastMessage sends a message to all room participants except the sender
func (c *ChatHandler) broadcastMessage(ctx context.Context, req *pb.ChatMessage, roomStreams map[string]chan *pb.ChatMessage) {
	var wg sync.WaitGroup
	for username, ch := range roomStreams {
		if username == req.Sender && req.Type != pb.MessageType_AES_KEY && req.Type != pb.MessageType_AES_KEY_UPDATE {
			continue
		}
		wg.Add(1)
		go func(user string, channel chan *pb.ChatMessage) {
			defer wg.Done()
			c.logInfo("[%s -> %s] %s", req.Sender, user, req.Content)
			select {
			case channel <- req:
			case <-ctx.Done():
				c.logError("Sending to %s in room %s canceled due to timeout", user, req.RoomId)
			case <-time.After(1 * time.Second):
				c.logError("Channel for %s in room %s is blocked", user, req.RoomId)
			}
		}(username, ch)
	}
	wg.Wait()
}

func (c *ChatHandler) SendMessage(ctx context.Context, req *pb.ChatMessage) (*pb.SendResponse, error) {
	c.logInfo("[%s -> room:%s] %s", req.Sender, req.RoomId, req.Content)
	var res *pb.SendResponse
	c.withLock(func() {
		var err error
		res, err = c.chatService.SendMessage(ctx, req.RoomId, req.Sender, req.Content)
		if err != nil {
			c.logError("Failed to send message in room %s: %v", req.RoomId, err)
			return
		}

		if req.Type == pb.MessageType_AES_KEY_UPDATE {
			if err := c.chatService.StoreEncryptedAESKey(ctx, req.RoomId, req.Content); err != nil {
				c.logError("Failed to store new AES key: %v", err)
			}
		}

		if roomStreams, exists := c.streams[req.RoomId]; exists {
			c.broadcastMessage(ctx, req, roomStreams)
		}
	})
	return res, nil
}

func (c *ChatHandler) ReceiveMessages(req *pb.ReceiveRequest, stream pb.Chat_ReceiveMessagesServer) error {
	roomId, username := req.RoomId, req.Username
	var msgChan chan *pb.ChatMessage
	c.withLock(func() {
		if streams, exists := c.streams[roomId]; exists {
			msgChan = streams[username]
		}
	})
	if msgChan == nil {
		return fmt.Errorf(errors.ErrUserNotInRoom.Error(), username, roomId)
	}

	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				c.logError("Channel for %s in room %s is closed", username, roomId)
				return nil
			}
			if err := stream.Send(msg); err != nil {
				c.logError("Client %s in room %s disconnected: %v", username, roomId, err)
				return nil
			}
		case <-stream.Context().Done():
			c.logError("Client %s in room %s disconnected (context done)", username, roomId)
			return nil
		}
	}
}

func (c *ChatHandler) CheckDuplicateUser(ctx context.Context, req *pb.CheckUserNameRequest) (*pb.CheckUserNameResponse, error) {
	return c.chatService.CheckDuplicateUser(ctx, req.RoomId, req.Username)
}

// QuitRoom removes a user from the room and promotes a new master if needed
func (c *ChatHandler) QuitRoom(ctx context.Context, req *pb.QuitRequest) (*pb.QuitResponse, error) {
	var res *pb.QuitResponse
	c.withLock(func() {
		var err error
		res, err = c.chatService.QuitRoom(ctx, req.Username, req.RoomId)
		if err != nil {
			c.logError("Failed to quit room %s for रोज़ %s: %v", req.RoomId, req.Username, err)
			return
		}

		master, err := c.chatService.FindMaster(ctx, req.RoomId)
		if err == nil && master == req.Username {
			newMaster, err := c.chatService.FindMaster(ctx, req.RoomId)
			if err == nil && newMaster != "" {
				c.logInfo("Master %s left, promoting %s as new master for room %s", req.Username, newMaster, req.RoomId)
				c.promoteNewMaster(req.RoomId, newMaster)
			}
		}

		if roomStreams, exists := c.streams[req.RoomId]; exists {
			if ch, ok := roomStreams[req.Username]; ok {
				close(ch)
				delete(roomStreams, req.Username)
			}
			if len(roomStreams) == 0 {
				delete(c.streams, req.RoomId)
			}
		}
	})
	return res, nil
}

func (c *ChatHandler) promoteNewMaster(roomId, newMaster string) {
	if roomStreams, exists := c.streams[roomId]; exists {
		if ch, ok := roomStreams[newMaster]; ok {
			ch <- &pb.ChatMessage{
				RoomId:    roomId,
				Sender:    "system",
				Content:   "promote_to_master",
				Type:      pb.MessageType_SYSTEM,
				Timestamp: time.Now().Unix(),
			}
		}
	}
}

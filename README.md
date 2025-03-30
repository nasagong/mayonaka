# 🌙 Mayonaka, Secure Midnight Conversations over Terminal

![main](https://github.com/user-attachments/assets/11d17cca-9bfc-4693-ab44-b1c163bfe348)


Mayonaka (真夜中) is an open-source terminal chat application that draws its name from the Japanese word for "midnight" while offering robust end-to-end encryption. 

The application empowers users to exchange messages through a server that is fundamentally incapable of intercepting or deciphering their communication contents. 

Leveraging gRPC—a high-performance communication framework that surpasses traditional REST APIs—Mayonaka delivers efficient, low-latency interactions, even in real-time multi-user environments. 

Like its namesake, the app provides a discreet and secure digital sanctuary where users can engage in private conversations, hidden from prying eyes under the metaphorical cover of night.

---

## Why gRPC ?

gRPC facilitates bi-directional streaming, which enables real-time message transmission without the need for continuous polling.

By leveraging HTTP/2, gRPC supports multiplexed connections, resulting in significantly lower latency compared to traditional REST architectures.

Message structures are defined with strong typing and can be automatically generated from .proto files, ensuring precise data communication.

These characteristics make gRPC an excellent choice for developing efficient and reliable terminal-based group chat systems.


## Key Features
- End-to-end encryption with AES and RSA.
- Automatic master election for uninterrupted key management.
- Real-time messaging with gRPC streams and Redis backend.

## Getting started (Server)
### Prerequisites
- **Go**: Version 1.23.5 or higher 

- **Redis**: A running Redis instance (install via redis.io or use Docker: docker run -d -p 6379:6379 redis).

- **gRPC Tools**: nstall protoc and Go plugins (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest and go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest).

### Steps

**1. Clone mayonaka**
```bash
git clone https://github.com/yourusername/mayonaka.git
cd mayonaka
```

**2. Install Dependencies**
```bash
go mod tidy
```

**3. Generate gRPC Code**
```bash
mkdir mayonaka/internal/pb
make proto_chat
``` 

**4. Run the server / redis**
```bash
make docker-dev
```

## Getting started (Client)

**1. Download**

You can download the pre-built binary from the [Releases](https://github.com/nasagong/mayonaka/releases) page.

**2. Run it**

To connect to a specific remote server, use the following command.
```bash
./mayonaka_{yourOs}_{yourArch} -server=your_server:50051

# Example: macOS
./mayonaka-darwin-arm64 -server=your_server:50051
```

or if you want just localhost environment, use the command below.

```bash
./mayonaka_{yourOs}_{yourArch}
```



## Overview

![overview](https://github.com/user-attachments/assets/b6228262-76bf-4cb8-814a-b443f518f9cf)



This project is a real-time chat server developed using Go and gRPC, with a core focus on secure communication through end-to-end encryption (E2EE). 

Here's the breakdown of its functionality: when a user sends a message, their client encrypts it with a shared AES key specific to the chat room, transforming something like 'Hello' into an unreadable ciphertext (such as 0x12ab34cd...). 

The server functions as a neutral relay, transparently forwarding this encrypted message to all room participants without ever decrypting its contents, thereby guaranteeing that only the clients—not the server—can access the actual message. On the recipient's side, each client utilizes the same AES key to decrypt the ciphertext back into the original message, 'Hello.'



![chat](https://github.com/user-attachments/assets/f284c1d9-b5b1-4779-b2c1-3a0557e25979)



The key exchange mechanism begins with each client generating an RSA key pair consisting of public and private keys. Upon joining a room, a user sends their RSA public key to the server, which then shares it with the room's 'master' (the first user to join). This is handled in the ```notifyMaster``` function

```go
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
```
The master then generates an AES key for the room, encrypts it using each participant's RSA public key, and distributes it through the server. Subsequently, each client uses their RSA private key to decrypt and obtain the AES key, effectively establishing a secure, shared secret among all room members.

If the current master disconnects, the system automatically elects a new master from the remaining participants to ensure continued secure key management and communication. This logic is implemented in the ```QuitRoom``` method

```go
func (c *ChatHandler) QuitRoom(ctx context.Context, req *pb.QuitRequest) (*pb.QuitResponse, error) {
    var res *pb.QuitResponse
    c.withLock(func() {
        res, err := c.chatService.QuitRoom(ctx, req.Username, req.RoomId)
        if err != nil {
            c.logError("Failed to quit room %s for %s: %v", req.RoomId, req.Username, err)
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
        // ..,
    })
    return res, nil
}
```



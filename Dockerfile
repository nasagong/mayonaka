FROM golang:1.23.5 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /mayonaka ./cmd/main.go

FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /mayonaka .

EXPOSE 50051

CMD ["./mayonaka"]
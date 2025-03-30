package status

type RoomStatus int

const (
	RoomActive RoomStatus = iota
	RoomNotFound
	RoomUnknownError
)

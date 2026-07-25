package domain

import (
	"errors"
	"time"
)

var ErrPresenceNotAllowed = errors.New("không có quyền xem presence trong workspace")

type Presence struct {
	UserID          string
	WorkspaceID     *string
	DeviceID        string
	SocketID        string
	NodeID          string
	Status          string
	LastHeartbeatAt time.Time
	ConnectedAt     time.Time
	Metadata        []byte
}

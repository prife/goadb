package wire

// Sender sends messages to the server.
type Sender interface {
	SendMessage(msg []byte) error
	// NewSyncSender() SyncSender
	// Close() error
}

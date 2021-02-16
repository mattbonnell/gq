package pkg

type MessageStatus uint8

const (
	MessageStatusQueued MessageStatus = iota
	MessageStatusConsumed
	MessageStatusProcessed
	MessageStatusFailed
)

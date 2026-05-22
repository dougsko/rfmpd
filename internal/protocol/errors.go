package protocol

import "errors"

var (
	ErrInvalidFrame     = errors.New("invalid protobuf frame")
	ErrUnknownFrameType = errors.New("unknown frame type")
)

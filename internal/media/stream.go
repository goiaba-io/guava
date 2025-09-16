package media

import (
	"io"
	"time"
)

type ActiveStream struct {
	ID       string
	Codec    string
	Filename string
	W        io.WriteCloser
	Written  int64
	OpenedAt time.Time
}

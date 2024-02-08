package internal

import "io"

type Email interface {
	SendAttachment(string, io.Reader) error
}

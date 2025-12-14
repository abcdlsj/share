package model

type EntryType int

const (
	EntryText EntryType = iota
	EntryImage
)

type Entry struct {
	Type EntryType
	Text string
	URL  string
}

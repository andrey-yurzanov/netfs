package message

// An event sent after changing of the terminal size.
type ResizeMsg struct {
	Width  int
	Height int
}

// An event is sent every N seconds.
type RefreshMsg struct{}

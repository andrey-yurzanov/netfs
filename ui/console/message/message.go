package message

// The event sends after changing the terminal size.
type ResizeMsg struct {
	Width  int
	Height int
}

// The event sends every N seconds.
type RefreshMsg struct{}

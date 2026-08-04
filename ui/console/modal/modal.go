package modal

import (
	"netfs/ui/console/message"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type OpenModalMsg struct {
	Name    string
	Payload any
}

type CloseModalMsg struct {
	Name    string
	Payload any
}

type ActionModalMsg struct {
	Action  string
	Payload string
}

// A button of the modal window.
type ModalButton struct {
	title    string
	shortcut string
	cmd      func(any) tea.Cmd
}

// The modal window.
type Modal interface {
	tea.Model

	// The function sets the visibility flag for the modal window.
	SetVisibled(bool)
	// The function returns the visibility flag for the modal window.
	GetVisibled() bool
	// The function sets the title of the modal window.
	SetTitle(string)
	// The function returns the title of the modal window.
	GetTitle() string
}

type ModalGroupView struct {
	modals   []Modal
	style    lipgloss.Style
	visibled bool
}

func (model *ModalGroupView) GetVisibled() bool {
	return model.visibled
}

func (model *ModalGroupView) Init() tea.Cmd {
	return nil
}

func (model *ModalGroupView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case message.ResizeMsg:
		model.style = model.
			style.
			Width(msg.Width).
			Height(msg.Height)
	}

	return model, cmd
}

func (model *ModalGroupView) View() string {
	return model.style.Render("")
}

func NewModalGroupView(modals ...Modal) *ModalGroupView {
	return &ModalGroupView{
		visibled: false,
		modals:   modals,
		style: lipgloss.
			NewStyle().
			AlignVertical(lipgloss.Center).
			AlignHorizontal(lipgloss.Center),
	}
}

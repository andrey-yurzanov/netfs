package modal

import (
	"netfs/ui/console/message"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalButtonType defines the set of available buttons for the modal windows.
type ModalButtonType uint8

const (
	// NONE represents an uninitialized button state.
	NONE ModalButtonType = iota

	// YES represents the confirmation action.
	YES

	// NO represents the cancellation action.
	NO
)

// OpenModalMsg represents a message used to trigger the opening of a modal window.
type OpenModalMsg struct {
	// Name is the unique name of the modal to be opened.
	Name string

	// Invoker indicates the component that triggered this message.
	Invoker string

	// Action describes the action for which the modal window was opened.
	Action string

	// Payload holds specific data for the modal window.
	Payload any
}

// CloseModalMsg represents a message used to trigger the closing of a modal window.
type CloseModalMsg struct {
	// Name is the unique name of the modal to be closed.
	Name string

	// Invoker indicates the component that triggered this message.
	Invoker string

	// Action describes the action for which the modal window was opened.
	Action string

	// Button represents the specific button that triggered the close event.
	Button any

	// Payload holds specific data for the modal window.
	Payload any
}

// ModalGroupViewItem represents an item within group of modals.
type ModalGroupViewItem struct {
	// Name is the unique name of the modal
	Name string

	// Modal is the view of the modal window.
	Modal tea.Model
}

// ModalGroupView manages a collection of modal windows.
type ModalGroupView struct {
	modals  []ModalGroupViewItem
	style   lipgloss.Style
	active  tea.Model
	visible bool
}

// Visible returns true if any modal element of the group is currently visible.
func (model *ModalGroupView) Visible() bool {
	return model.visible
}

func (model *ModalGroupView) Init() tea.Cmd {
	return nil
}

func (model *ModalGroupView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case OpenModalMsg:
		model.visible = true
		for _, item := range model.modals {
			if item.Name == msg.Name {
				model.active = item.Modal
				break
			}
		}

	case CloseModalMsg:
		model.visible = false
		model.active = nil

	case message.ResizeMsg:
		model.style = model.
			style.
			Width(msg.Width).
			Height(msg.Height)
	}

	if model.active != nil {
		model.active, cmd = model.active.Update(msg)
	}
	return model, cmd
}

func (model *ModalGroupView) View() string {
	return model.
		style.
		Render(model.active.View())
}

// NewModalGroupView creates a new instance of ModalGroupView.
func NewModalGroupView(modals ...ModalGroupViewItem) *ModalGroupView {
	return &ModalGroupView{
		visible: false,
		modals:  modals,
		style: lipgloss.
			NewStyle().
			AlignVertical(lipgloss.Center).
			AlignHorizontal(lipgloss.Center),
	}
}

package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInputModal is the name of TextInputModalView.
const TextInputModal = "TextInputModal"

// TextInputModalPayload represents the data of TextInputModalView.
type TextInputModalPayload struct {
	// Title is title of the modal.
	Title string

	// Value holds the text within the input field.
	Value string

	// Error contains the error message to be displayed to the user.
	Error string

	// Validator is a callback function to validate the user's input.
	Validator func(string) (bool, string)
}

// TextInputModalView represents a modal window with a text input field.
type TextInputModalView struct {
	input               textinput.Model
	titleStyle          lipgloss.Style
	buttonStyle         lipgloss.Style
	buttonDisableStyle  lipgloss.Style
	buttonSelectedStyle lipgloss.Style
	errorMessageStyle   lipgloss.Style
	windowStyle         lipgloss.Style
	action              string
	invoker             string
	name                string
	payload             TextInputModalPayload
	button              ModalButtonType
	valid               bool
}

func (model TextInputModalView) Init() tea.Cmd {
	return textinput.Blink
}

func (model TextInputModalView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd, inputCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		var yesPressed, noPressed, toInputPressed, toButtonsPressed bool
		if model.isYesShortcutPressed(msg) {
			yesPressed = true
		} else if model.isNoShortcutPressed(msg) {
			noPressed = true
		} else if model.button == NONE && msg.Type == tea.KeyDown {
			toButtonsPressed = true
		} else if msg.Type == tea.KeyUp {
			toInputPressed = true
		} else if model.button == NONE {
			model.input, inputCmd = model.input.Update(msg)
		}

		model.valid, model.payload.Error = model.validateInput()
		if model.valid && yesPressed {
			cmd = model.closeModal(YES, TextInputModalPayload{Value: model.input.Value()})
		} else if noPressed {
			cmd = model.closeModal(NO, TextInputModalPayload{})
		} else if toInputPressed {
			model.button = NONE
			model.input.Focus()
		} else if toButtonsPressed {
			if model.valid {
				model.button = YES
			} else {
				model.button = NO
			}
			model.input.Blur()
		} else if model.valid && model.button != NONE && msg.Type == tea.KeyLeft {
			model.button = YES
		} else if msg.Type == tea.KeyRight {
			model.button = NO
		}
	case OpenModalMsg:
		model.action = msg.Action
		model.invoker = msg.Invoker

		model.payload = msg.Payload.(TextInputModalPayload)
		model.input.SetValue(model.payload.Value)
	}

	return model, tea.Sequence(cmd, inputCmd)
}

func (model TextInputModalView) View() string {
	var yesButton, noButton string
	switch model.button {
	case YES:
		yesButton = model.buttonSelectedStyle.Render("Yes (alt+y)") // TODO. from settings.
		noButton = model.buttonStyle.Render("No (alt+n)")
	case NO:
		if model.valid {
			yesButton = model.buttonStyle.Render("Yes (alt+y)")
		} else {
			yesButton = model.buttonDisableStyle.Render("Yes (alt+y)")
		}

		noButton = model.buttonSelectedStyle.Render("No (alt+n)")
	default:
		if model.valid {
			yesButton = model.buttonStyle.Render("Yes (alt+y)")
		} else {
			yesButton = model.buttonDisableStyle.Render("Yes (alt+y)")
		}
		noButton = model.buttonStyle.Render("No (alt+n)")
	}

	return model.
		windowStyle.
		Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				model.titleStyle.Render(model.payload.Title),
				model.input.View(),
				lipgloss.JoinHorizontal(
					lipgloss.Center,
					yesButton,
					noButton,
				),
				"",
				model.errorMessageStyle.Render(model.payload.Error),
			),
		)
}

func (model TextInputModalView) isYesShortcutPressed(msg tea.KeyMsg) bool {
	return (msg.Type == tea.KeyEnter && model.button == YES) ||
		(model.input.Focused() && msg.Type == tea.KeyEnter) ||
		(msg.String() == "alt+y")
}

func (model TextInputModalView) isNoShortcutPressed(msg tea.KeyMsg) bool {
	return (msg.Type == tea.KeyEnter && model.button == NO) ||
		(msg.Type == tea.KeyEsc) ||
		(msg.String() == "alt+n")
}

func (model TextInputModalView) validateInput() (bool, string) {
	value := strings.TrimSpace(model.input.Value())
	if len(value) > 0 {
		return model.payload.Validator(value)
	}
	return false, ""
}

func (model TextInputModalView) closeModal(button ModalButtonType, payload TextInputModalPayload) tea.Cmd {
	return func() tea.Msg {
		return CloseModalMsg{
			Name:    model.name,
			Action:  model.action,
			Invoker: model.invoker,
			Button:  button,
			Payload: payload,
		}
	}
}

// NewTextInputModalView creates a new instance of TextInputModalView.
func NewTextInputModalView(name string) *TextInputModalView {
	input := textinput.New()
	input.Width = 20
	input.Focus()

	return &TextInputModalView{
		input: input,
		name:  name,
		titleStyle: lipgloss.
			NewStyle().
			Padding(1),
		buttonStyle: lipgloss.
			NewStyle().
			MarginRight(1).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			Foreground(lipgloss.Color("#fff")).
			BorderForeground(lipgloss.Color("#fff")),
		buttonDisableStyle: lipgloss.
			NewStyle().
			MarginRight(1).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			Foreground(lipgloss.Color("#6d6d6d")).
			BorderForeground(lipgloss.Color("#6d6d6d")),
		buttonSelectedStyle: lipgloss.
			NewStyle().
			MarginRight(1).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#3b82f6")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3b82f6")),
		errorMessageStyle: lipgloss.
			NewStyle().
			Foreground(lipgloss.Color("#f63b3b")),
		windowStyle: lipgloss.
			NewStyle().
			Padding(2).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#fff")),
	}
}

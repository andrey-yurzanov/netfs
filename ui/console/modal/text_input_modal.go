package modal

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TextInputModalView struct {
	input               textinput.Model
	titleStyle          lipgloss.Style
	buttonStyle         lipgloss.Style
	buttonSelectedStyle lipgloss.Style
	windowStyle         lipgloss.Style
	title               string
	action              string
	invoker             string
	button              ModalButtonType
}

func (model TextInputModalView) Init() tea.Cmd {
	return textinput.Blink
}

func (model TextInputModalView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if model.isOkButton(msg) {
			cmd = func() tea.Msg {
				return CloseModalMsg{
					Name:    ConfirmModal,
					Action:  model.action,
					Invoker: model.invoker,
					Button:  YES,
					Payload: model.input.Value(),
				}
			}
		} else if model.isCancelButton(msg) {
			cmd = func() tea.Msg {
				return CloseModalMsg{
					Name:    ConfirmModal,
					Action:  model.action,
					Invoker: model.invoker,
					Button:  NO,
				}
			}
		} else if msg.Type == tea.KeyUp {
			model.button = NONE
			model.input.Focus()
		} else if model.button == NONE && msg.Type == tea.KeyDown {
			model.button = YES
			model.input.Blur()
		} else if msg.Type == tea.KeyLeft {
			model.button = YES
		} else if msg.Type == tea.KeyRight {
			model.button = NO
		}
	case OpenModalMsg:
		model.title = msg.Payload.(string)
		model.action = msg.Action
		model.invoker = msg.Invoker
	}

	model.input, cmd = model.input.Update(msg)
	return model, cmd
}

func (model TextInputModalView) View() string {
	var yesButton, noButton string
	switch model.button {
	case YES:
		yesButton = model.buttonSelectedStyle.Render("Yes (alt+y)")
		noButton = model.buttonStyle.Render("No (alt+n)")
	case NO:
		yesButton = model.buttonStyle.Render("Yes (alt+y)")
		noButton = model.buttonSelectedStyle.Render("No (alt+n)")
	default:
		yesButton = model.buttonStyle.Render("Yes (alt+y)")
		noButton = model.buttonStyle.Render("No (alt+n)")
	}

	return model.
		windowStyle.
		Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				model.titleStyle.Render(model.title),
				model.input.View(),
				lipgloss.JoinHorizontal(
					lipgloss.Center,
					yesButton,
					noButton,
				),
			),
		)
}

func (model TextInputModalView) isOkButton(msg tea.KeyMsg) bool {
	return (msg.Type == tea.KeyEnter && model.button == YES) ||
		(model.input.Focused() && msg.Type == tea.KeyEnter) ||
		(msg.String() == "alt+y")
}

func (model TextInputModalView) isCancelButton(msg tea.KeyMsg) bool {
	return (msg.Type == tea.KeyEnter && model.button == NO) ||
		(msg.Type == tea.KeyEsc) ||
		(msg.String() == "alt+n")
}

func NewTextInputModalView() *TextInputModalView {
	input := textinput.New()
	input.Width = 20
	input.Focus()

	return &TextInputModalView{
		input: input,
		titleStyle: lipgloss.
			NewStyle().
			Padding(1),
		buttonStyle: lipgloss.
			NewStyle().
			MarginRight(1).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#fff")),
		buttonSelectedStyle: lipgloss.
			NewStyle().
			MarginRight(1).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#3b82f6")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3b82f6")),
		windowStyle: lipgloss.
			NewStyle().
			Padding(2).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#fff")),
	}
}

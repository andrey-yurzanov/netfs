package modal

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmModal is the name for the ConfirmModalView.
const ConfirmModal = "ConfirmModal"

// ConfirmModalView is a modal window for confirming an action.
type ConfirmModalView struct {
	windowStyle         lipgloss.Style
	titleStyle          lipgloss.Style
	buttonStyle         lipgloss.Style
	buttonSelectedStyle lipgloss.Style
	title               string
	action              string
	invoker             string
	button              ModalButtonType
}

func (model ConfirmModalView) Init() tea.Cmd {
	return nil
}

func (model ConfirmModalView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	return model, cmd
}

func (model ConfirmModalView) View() string {
	var yesButton, noButton string
	switch model.button {
	case YES:
		yesButton = model.buttonSelectedStyle.Render("Yes (alt+y)") // TODO. from settings.
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
				lipgloss.JoinHorizontal(
					lipgloss.Center,
					yesButton,
					noButton,
				),
			),
		)
}

func (model ConfirmModalView) isOkButton(msg tea.KeyMsg) bool {
	return (msg.Type == tea.KeyEnter && model.button == YES) || (msg.String() == "alt+y") // TODO. from settings.
}

func (model ConfirmModalView) isCancelButton(msg tea.KeyMsg) bool {
	return (msg.Type == tea.KeyEnter && model.button == NO) ||
		(msg.Type == tea.KeyEsc) ||
		(msg.String() == "alt+n") // TODO. from settings.
}

// NewConfirmModalView creates a new instance of ConfirmModalView.
func NewConfirmModalView() *ConfirmModalView {
	return &ConfirmModalView{
		button: YES,
		titleStyle: lipgloss.
			NewStyle().
			Padding(1),
		buttonStyle: lipgloss.
			NewStyle().
			MarginRight(1).
			Align(lipgloss.Center).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#fff")), // TODO. from settings.
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

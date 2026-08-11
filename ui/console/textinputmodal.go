package console

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The modal window.
type TextInputModal struct {
	input               textinput.Model
	titleStyle          lipgloss.Style
	buttonStyle         lipgloss.Style
	buttonSelectedStyle lipgloss.Style
	windowStyle         lipgloss.Style
	// buttons             []ModalButton
	title    string
	selected int
	visibled bool
}

// The function sets the visibility flag for the modal window.
func (model *TextInputModal) SetVisibled(value bool) {
	model.visibled = value
}

// The function returns the visibility flag for the modal window.
func (model *TextInputModal) GetVisibled() bool {
	return model.visibled
}

// The function replaces the title of the modal window.
func (model *TextInputModal) SetTitle(title string) {
	model.title = title
}

// // The function replaces the buttons of the modal window.
// func (model *TextInputModal) SetButtons(buttons []ModalButton) {
// 	model.buttons = buttons
// }

func (model *TextInputModal) Init() tea.Cmd {
	return textinput.Blink
}

func (model *TextInputModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// switch msg := msg.(type) {
	// case tea.KeyMsg:
	// 	if model.visibled {
	// 		switch msg.Type {
	// 		case tea.KeyLeft:
	// 			if model.selected > 0 {
	// 				model.selected -= 1
	// 			}
	// 		case tea.KeyRight:
	// 			if model.selected < len(model.buttons)-1 {
	// 				model.selected += 1
	// 			}
	// 		case tea.KeyEnter:
	// 			cmd = model.buttons[model.selected].cmd
	// 		default:
	// 			for _, button := range model.buttons {
	// 				if strings.EqualFold(button.shortcut, string(msg.Runes)) {
	// 					cmd = button.cmd
	// 					break
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	model.input, cmd = model.input.Update(msg)
	return model, cmd
}

func (model *TextInputModal) View() string {
	// buttons := make([]string, len(model.buttons))
	// for index, button := range model.buttons {
	// 	if index == model.selected {
	// 		buttons[index] = model.buttonSelectedStyle.Render(button.title)
	// 	} else {
	// 		buttons[index] = model.buttonStyle.Render(button.title)
	// 	}
	// }

	return model.
		windowStyle.
		Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				model.titleStyle.Render(model.title),
				model.input.View(),
				lipgloss.JoinHorizontal(
					lipgloss.Center,
					// buttons...,
				),
			),
		)
}

func NewTextInputModal() *TextInputModal {
	input := textinput.New()
	input.CharLimit = 156
	input.Width = 20
	input.Focus()

	modal := &TextInputModal{
		selected: 0,
		input:    input,
		// buttons:  []ModalButton{},
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
	return modal
}

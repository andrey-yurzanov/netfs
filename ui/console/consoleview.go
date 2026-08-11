package console

import (
	"netfs/api"
	"netfs/ui/console/message"
	"netfs/ui/console/modal"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TODO. from settings?
const EnterKeyMsg = "enter"
const BackspaceKeyMsg = "backspace"
const AltBackspaceKeyMsg = "alt+backspace"
const QuitKeyMsg = "alt+q"
const HostActiveKeyMsg = "alt+h"
const FileActiveKeyMsg = "alt+f"
const TaskActiveKeyMsg = "alt+t"

type ConsoleActiveView uint8

const (
	Host ConsoleActiveView = iota
	File
	Task
)

// The event sends after switching to another view.
type ChangeActiveViewMsg struct {
	View ConsoleActiveView
}

// The main view of the UI.
type ConsoleView struct {
	hostsView  tea.Model
	fileView   tea.Model
	taskView   tea.Model
	activeView ConsoleActiveView
	style      lipgloss.Style
	modalView  tea.Model
}

func (model ConsoleView) Init() tea.Cmd {
	return tea.Sequence(
		model.hostsView.Init(),
		model.fileView.Init(),
		model.taskView.Init(),
		model.modalView.Init(),
		func() tea.Msg { return ChangeActiveViewMsg{View: Host} },
		tea.Every(3*time.Second, func(t time.Time) tea.Msg { return message.RefreshMsg{} }), // TODO. 3*time.Second - from settings
	)
}

func (model ConsoleView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var hostViewCmd tea.Cmd
	var fileViewCmd tea.Cmd
	var taskViewCmd tea.Cmd
	var modalViewCmd tea.Cmd

	modalView := model.modalView.(*modal.ModalGroupView)
	switch msg := msg.(type) {
	case message.RefreshMsg:
		cmd = tea.Every(3*time.Second, func(t time.Time) tea.Msg { return message.RefreshMsg{} }) // TODO. 3*time.Second - from settings
		model.hostsView, hostViewCmd = model.hostsView.Update(msg)
		model.fileView, fileViewCmd = model.fileView.Update(msg)
		model.taskView, taskViewCmd = model.taskView.Update(msg)

	case tea.KeyMsg:
		// Quit
		if msg.String() == QuitKeyMsg {
			return model, tea.Quit
		}

		// Blocks input when modal is visible
		if modalView.Visible() {
			model.modalView, modalViewCmd = model.modalView.Update(msg)
		} else {
			// Switches to active view
			switch msg.String() {
			case HostActiveKeyMsg:
				cmd = func() tea.Msg { return ChangeActiveViewMsg{View: Host} }
			case FileActiveKeyMsg:
				cmd = func() tea.Msg { return ChangeActiveViewMsg{View: File} }
			case TaskActiveKeyMsg:
				cmd = func() tea.Msg { return ChangeActiveViewMsg{View: Task} }
			default:
				// Propagates input to active view
				switch model.activeView {
				case Host:
					model.hostsView, hostViewCmd = model.hostsView.Update(msg)
				case File:
					model.fileView, fileViewCmd = model.fileView.Update(msg)
				case Task:
					model.taskView, taskViewCmd = model.taskView.Update(msg)
				}
			}
		}

	case ChangeActiveViewMsg:
		switch msg.View {
		case Host:
			model.activeView = Host
		case File:
			model.activeView = File
		case Task:
			model.activeView = Task
		}
		model.hostsView, hostViewCmd = model.hostsView.Update(msg)
		model.fileView, fileViewCmd = model.fileView.Update(msg)
		model.taskView, taskViewCmd = model.taskView.Update(msg)

	case tea.WindowSizeMsg:
		frameX, frameY := model.style.GetFrameSize()
		width := float32(msg.Width - frameX)
		height := float32(msg.Height - frameY)
		model.style = model.
			style.
			Width(int(width)).
			Height(int(height))

		// TODO. from settings?
		hostViewWidth := (width / 100.0) * 30.0
		fileViewWidth := int(width - hostViewWidth)
		fileViewHeight := int((height / 100.0) * 70.0)

		model.hostsView, hostViewCmd = model.hostsView.Update(message.ResizeMsg{Width: int(hostViewWidth), Height: int(height)})
		model.fileView, fileViewCmd = model.fileView.Update(message.ResizeMsg{Width: fileViewWidth, Height: fileViewHeight})
		model.taskView, taskViewCmd = model.taskView.Update(message.ResizeMsg{Width: fileViewWidth, Height: int(height) - fileViewHeight})
		model.modalView, modalViewCmd = model.modalView.Update(message.ResizeMsg{Width: int(width), Height: int(height)})
	case modal.OpenModalMsg:
		model.modalView, modalViewCmd = model.modalView.Update(msg)
		model.hostsView, hostViewCmd = model.hostsView.Update(msg)
		model.fileView, fileViewCmd = model.fileView.Update(msg)
		model.taskView, taskViewCmd = model.taskView.Update(msg)
	case modal.CloseModalMsg:
		model.modalView, modalViewCmd = model.modalView.Update(msg)
		model.hostsView, hostViewCmd = model.hostsView.Update(msg)
		model.fileView, fileViewCmd = model.fileView.Update(msg)
		model.taskView, taskViewCmd = model.taskView.Update(msg)
	default:
		// Blocks input when modal is visible
		if modalView.Visible() {
			model.modalView, modalViewCmd = model.modalView.Update(msg)
		} else {
			model.hostsView, hostViewCmd = model.hostsView.Update(msg)
			model.fileView, fileViewCmd = model.fileView.Update(msg)
			model.taskView, taskViewCmd = model.taskView.Update(msg)
		}
	}

	return model, tea.Sequence(cmd, hostViewCmd, fileViewCmd, taskViewCmd, modalViewCmd)
}

func (model ConsoleView) View() string {
	modal := model.modalView.(*modal.ModalGroupView)
	if modal.Visible() {
		return modal.View()
	}

	return model.style.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			model.hostsView.View(),
			lipgloss.JoinVertical(
				lipgloss.Left,
				model.fileView.View(),
				model.taskView.View(),
			),
		),
	)
}

// The function returns new instance of ConsoleView.
func NewConsoleViewModel(network *api.Network) tea.Model {
	style := lipgloss.
		NewStyle().
		Align(lipgloss.Left, lipgloss.Left)

	return ConsoleView{
		style:     style,
		hostsView: NewHostView(network),
		fileView:  NewFileView(network),
		taskView:  NewTaskView(network),
		modalView: modal.NewModalGroupView(
			modal.ModalGroupViewItem{Name: modal.ConfirmModal, Modal: modal.NewConfirmModalView()},
			modal.ModalGroupViewItem{Name: "CreateModal", Modal: modal.NewTextInputModalView()},
		),
	}
}

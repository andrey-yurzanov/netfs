package console

import (
	"io"
	"netfs/api"
	"netfs/ui/console/message"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The event sends after the host is selected.
type ChangeActiveHostMsg struct {
	Host  *api.RemoteHost
	Error error
}

// The event sends after receiving the hosts.
type ChangeHostsMsg struct {
	Items []list.Item
	Error error
}

type HostViewItem struct {
	Host *api.RemoteHost
}

func (item HostViewItem) Title() string       { return item.Host.Name }
func (item HostViewItem) Description() string { return item.Host.IP.String() }
func (item HostViewItem) FilterValue() string { return item.Host.Name }

type HostViewItemDelegate struct {
	itemStyle         lipgloss.Style
	itemSelectedStyle lipgloss.Style
}

func (delegate HostViewItemDelegate) Render(writer io.Writer, model list.Model, index int, item list.Item) {
	style := delegate.itemStyle
	if model.Index() == index {
		style = delegate.itemSelectedStyle
	}

	hostItem := item.(*HostViewItem)
	writer.Write(
		[]byte(
			style.Render(
				strings.Join([]string{hostItem.Host.Name, "(", hostItem.Host.IP.String(), ")"}, ""),
			),
		),
	)
}

func (HostViewItemDelegate) Height() int { return 1 }

func (HostViewItemDelegate) Spacing() int { return 0 }

func (HostViewItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

// The view for displaying hosts.
type HostView struct {
	list        list.Model
	style       lipgloss.Style
	activeStyle lipgloss.Style
	network     *api.Network
	active      bool
}

func (model HostView) Init() tea.Cmd {
	return func() tea.Msg {
		hosts, err := model.network.Hosts()
		if err == nil && len(hosts) > 0 {
			items := make([]list.Item, len(hosts))
			for index, host := range hosts {
				items[index] = &HostViewItem{Host: &host}
			}
			return ChangeHostsMsg{Items: items}
		}
		return ChangeHostsMsg{Error: err}
	}
}

func (model HostView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			item := model.list.SelectedItem()
			cmd = tea.Sequence(
				func() tea.Msg { return ChangeActiveHostMsg{Host: item.(*HostViewItem).Host} },
				func() tea.Msg { return ChangeActiveViewMsg{View: File} },
			)
		}

	case ChangeActiveViewMsg:
		model.active = (msg.View == Host)
	case ChangeHostsMsg:
		cmd = model.list.SetItems(msg.Items)
	case message.ResizeMsg:
		frameX, frameY := model.style.GetFrameSize()
		width := msg.Width - frameX
		height := msg.Height - frameY

		model.style = model.
			style.
			Width(width).
			Height(height)

		model.activeStyle = model.
			activeStyle.
			Width(width).
			Height(height)

		model.list.SetSize(width, height)
	}

	var listCmd tea.Cmd
	model.list, listCmd = model.list.Update(msg)
	return model, tea.Sequence(cmd, listCmd)
}

func (model HostView) View() string {
	if model.active {
		return model.activeStyle.Render(model.list.View())
	}
	return model.style.Render(model.list.View())
}

func NewHostView(network *api.Network) tea.Model {
	delegate := HostViewItemDelegate{
		itemStyle:         lipgloss.NewStyle(),
		itemSelectedStyle: lipgloss.NewStyle().Background(lipgloss.Color("#3b82f6")),
	}

	lst := list.New([]list.Item{}, delegate, 0, 0)
	lst.DisableQuitKeybindings()
	lst.SetShowFilter(false)
	lst.SetShowHelp(false)
	lst.SetShowTitle(false)
	lst.SetShowStatusBar(false)
	lst.SetShowPagination(false)

	style := lipgloss.
		NewStyle().
		Align(lipgloss.Left, lipgloss.Left).
		BorderForeground(lipgloss.Color("#fff")).
		BorderStyle(lipgloss.NormalBorder())

	activeStyle := lipgloss.
		NewStyle().
		Align(lipgloss.Left, lipgloss.Left).
		BorderForeground(lipgloss.Color("#3b82f6")).
		BorderStyle(lipgloss.NormalBorder())

	return &HostView{list: lst, network: network, style: style, activeStyle: activeStyle}
}

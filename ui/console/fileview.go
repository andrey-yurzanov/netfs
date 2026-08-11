package console

import (
	"io"
	"netfs/api"
	"netfs/ui/console/message"
	"netfs/ui/console/modal"
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const COUNT_MAX_LEN = 5
const COLUMN_TYPE_WIDTH = 5
const COLUMN_SIZE_WIDTH = 15
const TOO_LONG_LINE_POSTFIX = "..."

var TOO_LONG_LINE_POSTFIX_WIDTH = lipgloss.Width(TOO_LONG_LINE_POSTFIX)

type UpdateFilesMsg struct {
	Items []list.Item
}

type FileViewHistoryNode struct {
	Item list.Item
	Prev *FileViewHistoryNode
}

type FileViewItem struct {
	File *api.RemoteFile
}

func (item FileViewItem) Title() string       { return item.File.Info.Name }
func (item FileViewItem) Description() string { return item.File.Info.Path }
func (item FileViewItem) FilterValue() string { return item.File.Info.Name }

type FileViewItemDelegate struct {
	columnTypeStyle   lipgloss.Style
	columnNameStyle   lipgloss.Style
	columnSizeStyle   lipgloss.Style
	itemStyle         lipgloss.Style
	itemSelectedStyle lipgloss.Style
	isActive          bool
}

func (delegate FileViewItemDelegate) Render(writer io.Writer, model list.Model, index int, item list.Item) {
	style := delegate.itemStyle
	if delegate.isActive && model.Index() == index {
		style = delegate.itemSelectedStyle
	}

	fileItem := item.(*FileViewItem)
	nameColumn := fileItem.File.Info.Name
	nameWidth := delegate.columnNameStyle.GetWidth()
	if lipgloss.Width(nameColumn) > nameWidth {
		nameColumn = lipgloss.
			NewStyle().
			MaxWidth(nameWidth-TOO_LONG_LINE_POSTFIX_WIDTH).
			Render(nameColumn) + TOO_LONG_LINE_POSTFIX
	}

	writer.Write(
		[]byte(
			style.Render(
				lipgloss.JoinHorizontal(
					lipgloss.Left,
					delegate.columnTypeStyle.Render(fileItem.File.Info.Type.String()),
					delegate.columnNameStyle.Render(nameColumn),
					delegate.columnSizeStyle.Render(fileItem.File.Info.Size.String()),
				),
			),
		),
	)
}

func (FileViewItemDelegate) Height() int { return 1 }

func (FileViewItemDelegate) Spacing() int { return 0 }

func (FileViewItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

type FileView struct {
	list     list.Model
	style    lipgloss.Style
	id       string
	delegate *FileViewItemDelegate
	prev     *FileViewHistoryNode
	host     *api.RemoteHost
	network  *api.Network
	toCopy   *api.RemoteFile
}

func (model FileView) Init() tea.Cmd {
	return nil
}

func (model FileView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var headerCmd tea.Cmd
	var footerCmd tea.Cmd
	var listCmd tea.Cmd
	var modalCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		// Enter to the selected directory.
		case tea.KeyEnter:
			item := model.list.SelectedItem()
			file := item.(*FileViewItem).File
			if file.Info.Type == api.DIRECTORY {
				prev := FileViewHistoryNode{Item: item, Prev: model.prev}
				model.prev = &prev
				cmd = model.resolveFileChildren(file)
			}
		case tea.KeyBackspace:
			// Exit to the root directory of the selected host.
			if msg.Alt {
				cmd = func() tea.Msg { return ChangeActiveHostMsg{Host: model.host} }
				// Exit from the selected directory.
			} else if model.prev.Prev != nil {
				model.prev = model.prev.Prev
				item := model.prev.Item
				if item == nil {
					cmd = func() tea.Msg { return ChangeActiveHostMsg{Host: model.host} }
				} else {
					cmd = model.resolveFileChildren(item.(*FileViewItem).File)
				}
			}
		// Marks the file for copying.
		case tea.KeyCtrlC:
			item := model.list.SelectedItem()
			if _, ok := item.(*FileViewItem); ok {
				model.toCopy = item.(*FileViewItem).File
			}
		// Starts the file copying.
		case tea.KeyCtrlV:
			if model.toCopy != nil && model.prev.Item != nil {
				cmd = model.copyFile(false)
			}
		case tea.KeyDelete:
			item := model.list.SelectedItem()
			if _, ok := item.(*FileViewItem); ok {
				cmd = func() tea.Msg {
					return modal.OpenModalMsg{Name: modal.ConfirmModal, Invoker: model.id, Payload: item.(*FileViewItem).File.Info.Name}
				}
			}
		}
	case modal.CloseModalMsg:
		if msg.Invoker == model.id {
			if msg.Name == modal.ConfirmModal && msg.Button == modal.YES {
				cmd = model.deleteFile()
			}
			// TODO. add for copy
		}
	case ChangeActiveHostMsg:
		model.prev = &FileViewHistoryNode{}
		model.host = msg.Host
		cmd = model.resolveFileChildren(msg.Host.Root())
	case UpdateFilesMsg:
		cmd = model.list.SetItems(msg.Items)
	case ChangeActiveViewMsg:
		if msg.View == File {
			model.delegate.isActive = true
			model.style = model.style.BorderForeground(lipgloss.Color("#3b82f6"))
		} else {
			model.delegate.isActive = false
			model.style = model.style.BorderForeground(lipgloss.Color("#ffffff"))
		}
	case message.ResizeMsg:
		frameX, frameY := model.style.GetFrameSize()
		width := msg.Width - frameX
		height := msg.Height - frameY
		model.style = model.
			style.
			Width(width).
			Height(height)

		delegate := model.delegate
		delegate.columnTypeStyle = delegate.columnTypeStyle.Width(COLUMN_TYPE_WIDTH)
		delegate.columnNameStyle = delegate.columnNameStyle.Width(width - (COLUMN_TYPE_WIDTH + COLUMN_SIZE_WIDTH))
		delegate.columnSizeStyle = delegate.columnSizeStyle.Width(COLUMN_SIZE_WIDTH)
		delegate.itemStyle = delegate.itemStyle.Width(width)
		delegate.itemSelectedStyle = delegate.itemSelectedStyle.Width(width)

		model.list.SetSize(width, height)
	}
	model.list, listCmd = model.list.Update(msg)

	return model, tea.Sequence(cmd, headerCmd, footerCmd, listCmd, modalCmd)
}

func (model FileView) View() string {
	return model.style.Render(model.list.View())
}

func (model FileView) resolveFileChildren(file *api.RemoteFile) tea.Cmd {
	return func() tea.Msg {
		// TODO. Show error.
		children, _ := file.Children(model.network.Transport())
		items := make([]list.Item, len(children))
		for index, file := range children {
			items[index] = &FileViewItem{File: &file}
		}
		return UpdateFilesMsg{Items: items}
	}
}

func (model FileView) copyFile(replace bool) tea.Cmd {
	return func() tea.Msg {
		client := model.network.Transport()
		item := model.prev.Item.(*FileViewItem)
		file := model.toCopy
		path := filepath.Join(item.File.Info.Path, file.Info.Name)
		target := api.RemoteFile{
			Host: *model.host,
			Info: api.FileInfo{
				Id:   api.FileId(path),
				Name: file.Info.Name,
				Path: path,
				Type: file.Info.Type,
				Size: file.Info.Size,
			},
		}

		var err error
		if !replace {
			_, err = model.host.File(client, api.FileId(target.Info.Path))
			if err == nil { // File already exists.
				return modal.OpenModalMsg{Name: modal.ConfirmModal, Invoker: model.id, Payload: target.Info.Name}
			} else { // File not exists.
				_, err = file.CopyTo(model.network.Transport(), target)
			}
		} else {
			_, err = file.CopyTo(model.network.Transport(), target)
		}

		// TODO. it does not working!
		children, _ := item.File.Children(model.network.Transport())
		items := make([]list.Item, len(children))
		for index, file := range children {
			items[index] = &FileViewItem{File: &file}
		}
		return UpdateFilesMsg{Items: items}
	}
}

func (model FileView) deleteFile() tea.Cmd {
	return func() tea.Msg {
		item := model.list.SelectedItem()
		if _, ok := item.(*FileViewItem); ok {
			file := item.(*FileViewItem).File
			file.Remove(model.network.Transport())

			item = model.prev.Item
			if _, ok := item.(*FileViewItem); ok {
				file = item.(*FileViewItem).File
				children, _ := file.Children(model.network.Transport())
				items := make([]list.Item, len(children))
				for index, file := range children {
					items[index] = &FileViewItem{File: &file}
				}
				return UpdateFilesMsg{Items: items}
			}
		}
		return nil
	}
}

func NewFileView(network *api.Network) tea.Model {
	view := FileView{id: "", network: network}
	view.delegate = &FileViewItemDelegate{
		columnTypeStyle:   lipgloss.NewStyle().AlignHorizontal(lipgloss.Left),
		columnNameStyle:   lipgloss.NewStyle().AlignHorizontal(lipgloss.Left),
		columnSizeStyle:   lipgloss.NewStyle().AlignHorizontal(lipgloss.Right),
		itemStyle:         lipgloss.NewStyle(),
		itemSelectedStyle: lipgloss.NewStyle().Background(lipgloss.Color("#3b82f6")),
	}

	lst := list.New([]list.Item{}, view.delegate, 0, 0)
	lst.DisableQuitKeybindings()
	lst.SetShowFilter(false)
	lst.SetShowHelp(false)
	lst.SetShowTitle(false)
	lst.SetShowStatusBar(false)
	lst.SetShowPagination(false)
	view.list = lst

	view.style = lipgloss.
		NewStyle().
		Align(lipgloss.Left, lipgloss.Left).
		BorderForeground(lipgloss.Color("#ffffff")).
		BorderStyle(lipgloss.NormalBorder())

	return view
}

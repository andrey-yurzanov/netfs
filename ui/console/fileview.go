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

const deleteFileAction = "DeleteFile"
const copyFileAction = "CopyFile"
const moveFileAction = "MoveFile"
const createFileAction = "CreateFile"
const createDirectoryAction = "CreateDirectory"
const renameFileAction = "RenameFileAction"

var TOO_LONG_LINE_POSTFIX_WIDTH = lipgloss.Width(TOO_LONG_LINE_POSTFIX)

type UpdateFilesMsg struct {
	Items []list.Item
}

type FileViewHistoryNode struct {
	Item list.Item
	Prev *FileViewHistoryNode
}

type FileViewItem struct {
	File *api.File
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
	count    int
	list     list.Model
	style    lipgloss.Style
	id       string
	delegate *FileViewItemDelegate
	prev     *FileViewHistoryNode
	host     *api.Host
	network  *api.Network
	toCopy   *api.File
	toMove   *api.File
}

func (model FileView) Init() tea.Cmd {
	return nil
}

func (model FileView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd, headerCmd, footerCmd, listCmd, modalCmd tea.Cmd

	// Copy file and directory.
	if model.isCopyKeyPressed(msg) {
		model.toCopy = model.selectedItem()
	} else if model.isPasteKeyPressed(msg) {
		if model.toCopy != nil && model.prev.Item != nil {
			cmd = model.copyFile(false)
		}
	} else if model.isCopyFileConfirm(msg) {
		cmd = model.copyFile(true)
	}

	// Move file and directory.
	if model.isMoveKeyPressed(msg) {
		model.toCopy = nil
		model.toMove = model.selectedItem()
	} else if model.isPasteKeyPressed(msg) {
		if model.toMove != nil && model.prev.Item != nil {
			cmd = model.moveFile(model.toMove, false)
			model.toMove = nil
		}
	} else if model.isMoveFileConfirm(msg) {
		cmd = model.moveFile(model.toMove, true)
		model.toMove = nil
	}

	// Create files and directories.
	if model.isCreateFileKeyPressed(msg) {
		cmd = func() tea.Msg {
			return modal.OpenModalMsg{
				Name:    modal.TextInputModal,
				Action:  createFileAction,
				Invoker: model.id,
				Payload: modal.TextInputModalPayload{Title: "Create a new file?", Validator: model.checkFileExistsInList},
			}
		}
	} else if model.isCreateDirectoryKeyPressed(msg) {
		cmd = func() tea.Msg {
			return modal.OpenModalMsg{
				Name:    modal.TextInputModal,
				Action:  createDirectoryAction,
				Invoker: model.id,
				Payload: modal.TextInputModalPayload{Title: "Create a new directory?", Validator: model.checkFileExistsInList},
			}
		}
	} else if model.isCreateFileConfirm(msg) {
		cmd = model.createFile(msg, api.FILE)
	} else if model.isCreateDirectoryConfirm(msg) {
		cmd = model.createFile(msg, api.DIRECTORY)
	}

	if model.isRenameKeyPressed(msg) {
		cmd = func() tea.Msg {
			item := model.selectedItem()
			return modal.OpenModalMsg{
				Name:    modal.TextInputModal,
				Action:  renameFileAction,
				Invoker: model.id,
				Payload: modal.TextInputModalPayload{Title: "Rename the file?", Value: item.Info.Name, Validator: model.checkFileExistsInList},
			}
		}
	} else if model.isRenameFileConfirm(msg) {
		item := model.selectedItem()
		cmd = model.renameFile(item, msg)
	}

	if model.isDeleteKeyPressed(msg) {
		item := model.selectedItem()
		cmd = func() tea.Msg {
			return modal.OpenModalMsg{Name: modal.ConfirmModal, Action: deleteFileAction, Invoker: model.id, Payload: item.Info.Name}
		}
	} else if model.isDeleteFileConfirm(msg) {
		cmd = model.deleteFile()
	}

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
			model.style = model.style.BorderForeground(lipgloss.Color("#3b82f6")) // TODO. from settings
		} else {
			model.delegate.isActive = false
			model.style = model.style.BorderForeground(lipgloss.Color("#ffffff")) // TODO. from settings
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

func (model FileView) selectedItem() *api.File {
	item := model.list.SelectedItem()
	return item.(*FileViewItem).File
}

func (model FileView) isCopyKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "alt+c" // TODO. from settings
	}
	return false
}

func (model FileView) isMoveKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "alt+x" // TODO. from settings
	}
	return false
}

func (model FileView) isPasteKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "alt+v" // TODO. from settings
	}
	return false
}

func (model FileView) isCreateFileKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "alt+n" // TODO. from settings
	}
	return false
}

func (model FileView) isCreateDirectoryKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "alt+d" // TODO. from settings
	}
	return false
}

func (model FileView) isCreateFileConfirm(msg tea.Msg) bool {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		return msg.Invoker == model.id &&
			msg.Name == modal.TextInputModal &&
			msg.Action == createFileAction &&
			msg.Button == modal.YES
	}
	return false
}

func (model FileView) isCreateDirectoryConfirm(msg tea.Msg) bool {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		return msg.Invoker == model.id &&
			msg.Name == modal.TextInputModal &&
			msg.Action == createDirectoryAction &&
			msg.Button == modal.YES
	}
	return false
}

func (model FileView) isRenameKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "alt+g" // TODO. from settings
	}
	return false
}

func (model FileView) isRenameFileConfirm(msg tea.Msg) bool {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		return msg.Invoker == model.id &&
			msg.Name == modal.TextInputModal &&
			msg.Action == renameFileAction &&
			msg.Button == modal.YES
	}
	return false
}

func (model FileView) isDeleteKeyPressed(msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		return msg.String() == "delete" // TODO. from settings
	}
	return false
}

func (model FileView) isDeleteFileConfirm(msg tea.Msg) bool {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		return msg.Invoker == model.id &&
			msg.Name == modal.ConfirmModal &&
			msg.Action == deleteFileAction &&
			msg.Button == modal.YES
	}
	return false
}

func (model FileView) isCopyFileConfirm(msg tea.Msg) bool {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		return msg.Invoker == model.id &&
			msg.Name == modal.ConfirmModal &&
			msg.Action == copyFileAction &&
			msg.Button == modal.YES

	}
	return false
}

func (model FileView) isMoveFileConfirm(msg tea.Msg) bool {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		return msg.Invoker == model.id &&
			msg.Name == modal.ConfirmModal &&
			msg.Action == moveFileAction &&
			msg.Button == modal.YES

	}
	return false
}

func (model FileView) renameFile(file *api.File, msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if msg, ok := msg.(modal.CloseModalMsg); ok {
			name := msg.Payload.(modal.TextInputModalPayload).Value
			file.Rename(name) // TODO. show error

			file = model.prev.Item.(*FileViewItem).File
			children, _ := file.Children()
			items := make([]list.Item, len(children))
			for index, file := range children {
				items[index] = &FileViewItem{File: &file}
			}
			return UpdateFilesMsg{Items: items}
		}
		return nil
	}
}

func (model FileView) resolveFileChildren(file *api.File) tea.Cmd {
	return func() tea.Msg {
		// TODO. Show error.
		children, _ := file.Children()
		items := make([]list.Item, len(children))
		for index, file := range children {
			items[index] = &FileViewItem{File: &file}
		}
		return UpdateFilesMsg{Items: items}
	}
}

func (model FileView) copyFile(replace bool) tea.Cmd {
	return func() tea.Msg {
		item := model.prev.Item.(*FileViewItem)
		file := model.toCopy
		path := filepath.Join(item.File.Info.Path, file.Info.Name)
		target := api.File{
			Host: model.host,
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
			_, err = model.host.File(api.FileId(target.Info.Path))
			if err == nil { // File already exists.
				return modal.OpenModalMsg{
					Name:    modal.ConfirmModal,
					Action:  copyFileAction,
					Invoker: model.id,
					Payload: target.Info.Name,
				}
			} else { // File not exists.
				_, err = file.CopyTo(target)
			}
		} else {
			_, err = file.CopyTo(target)
		}

		children, _ := item.File.Children()
		items := make([]list.Item, len(children))
		for index, file := range children {
			items[index] = &FileViewItem{File: &file}
		}
		return UpdateFilesMsg{Items: items}
	}
}

func (model FileView) moveFile(file *api.File, replace bool) tea.Cmd {
	return func() tea.Msg {
		item := model.prev.Item.(*FileViewItem)
		path := filepath.Join(item.File.Info.Path, file.Info.Name)
		target := api.File{
			Host: model.host,
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
			_, err = model.host.File(api.FileId(target.Info.Path))
			if err == nil { // File already exists.
				return modal.OpenModalMsg{
					Name:    modal.ConfirmModal,
					Action:  moveFileAction,
					Invoker: model.id,
					Payload: target.Info.Name,
				}
			} else { // File not exists.
				_, err = file.MoveTo(target)
			}
		} else {
			_, err = file.MoveTo(target)
		}

		children, _ := item.File.Children()
		items := make([]list.Item, len(children))
		for index, file := range children {
			items[index] = &FileViewItem{File: &file}
		}
		return UpdateFilesMsg{Items: items}
	}
}

func (model FileView) deleteFile() tea.Cmd {
	return func() tea.Msg {
		file := model.selectedItem()
		file.Remove()

		file = model.prev.Item.(*FileViewItem).File
		children, _ := file.Children()
		items := make([]list.Item, len(children))
		for index, file := range children {
			items[index] = &FileViewItem{File: &file}
		}
		return UpdateFilesMsg{Items: items}
	}
}

func (model FileView) createFile(msg tea.Msg, fileType api.FileType) tea.Cmd {
	if msg, ok := msg.(modal.CloseModalMsg); ok {
		payload := msg.Payload.(modal.TextInputModalPayload)
		return func() tea.Msg {
			item := model.prev.Item.(*FileViewItem)
			path := filepath.Join(item.File.Info.Path, payload.Value)
			target := api.File{
				Host: model.host,
				Info: api.FileInfo{
					Id:   api.FileId(path),
					Name: payload.Value,
					Path: path,
					Type: fileType,
				},
			}

			_, err := model.host.File(api.FileId(target.Info.Path))
			if err == nil {
				if fileType == api.FILE {
					return modal.OpenModalMsg{
						Name:    modal.TextInputModal,
						Action:  createFileAction,
						Invoker: model.id,
						Payload: modal.TextInputModalPayload{Title: "Create a new file?", Error: payload.Value + " already exists!", Validator: model.checkFileExistsInList},
					}
				} else {
					return modal.OpenModalMsg{
						Name:    modal.TextInputModal,
						Action:  createDirectoryAction,
						Invoker: model.id,
						Payload: modal.TextInputModalPayload{Title: "Create a new directory?", Error: payload.Value + " already exists!", Validator: model.checkFileExistsInList},
					}
				}
			} else {
				_, err = target.Host.Create(target.Info, false)
				if err != nil {
					panic(err) // TODO. show error
				}
			}

			children, _ := item.File.Children()
			items := make([]list.Item, len(children))
			for index, file := range children {
				items[index] = &FileViewItem{File: &file}
			}
			return UpdateFilesMsg{Items: items}
		}
	}
	return nil
}

func (model FileView) checkFileExistsInList(name string) (bool, string) {
	items := model.list.Items()
	for _, item := range items {
		file := item.(*FileViewItem).File
		if file.Info.Name == name {
			return false, name + " already exists!"
		}
	}
	return true, ""
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

	return &view
}

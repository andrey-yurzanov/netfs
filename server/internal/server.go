package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"netfs/api"
	"netfs/api/transport"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const rootDirectory = "/"
const defaultRoot = "./"
const defaultPort = 8989
const defaultTimeout = 2 * time.Second
const defaultProtocol = transport.HTTP

const DefaultConfigPath = "./netfs_config.json"

var ErrFileAlreadyExists = errors.New("file already exists")
var ErrTooManyActiveTasks = errors.New("too many active tasks")
var ErrConfigIsEmpty = errors.New("configuration file is empty")

// The netfs logging configuration.
type ServerLogConfig struct {
	Level slog.Level
}

// The netfs server configuration.
type ServerConfig struct {
	Path     string `json:"-"`
	Log      ServerLogConfig
	Network  api.NetworkConfig
	RootList []string
}

// The function creates the default configuration.
func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		Path:     DefaultConfigPath,
		Log:      ServerLogConfig{Level: slog.LevelInfo},
		Network:  api.NetworkConfig{Port: defaultPort, Protocol: defaultProtocol, Timeout: defaultTimeout},
		RootList: []string{defaultRoot},
	}
}

// The function reads the configuration from the specified path.
func ReadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) == 0 {
			err = ErrConfigIsEmpty
		} else {
			config := &ServerConfig{}
			if err = json.Unmarshal(data, config); err == nil {
				config.Path = path
				return config, nil
			}
		}
	}
	return nil, err
}

// The function writes the configuration to the specified path.
func WriteServerConfig(config *ServerConfig) (*ServerConfig, error) {
	data, err := json.Marshal(config)
	if err == nil {
		err = os.WriteFile(config.Path, data, fs.ModePerm)
	}
	return config, err
}

// The netfs server.
type Server struct {
	rootList      []api.FileInfo
	copyScheduler *CopyScheduler
	log           *slog.Logger
	network       *api.Network
	receiver      transport.TransportReceiver
	stop          chan os.Signal
}

// Starts the netfs server.
func (srv *Server) Start() error {
	srv.receiver.Receive(api.Endpoints.ServerStop, srv.StopServerHandle)
	srv.receiver.Receive(api.Endpoints.ServerHost, srv.ServerHostHandle)
	srv.receiver.Receive(api.Endpoints.FileInfo.Name, srv.FileInfoHandle)
	srv.receiver.Receive(api.Endpoints.FileChildren.Name, srv.FileChildrenHandle)
	srv.receiver.Receive(api.Endpoints.FileCreate.Name, srv.FileCreateHandle)
	srv.receiver.Receive(api.Endpoints.FileWrite.Name, srv.FileWriteHandle)
	srv.receiver.Receive(api.Endpoints.FileRemove.Name, srv.FileRemoveHandle)
	srv.receiver.Receive(api.Endpoints.FileCopyStart, srv.FileCopyStartHandle)
	srv.receiver.Receive(api.Endpoints.FileCopy, srv.FileCopyHandle)
	// srv.receiver.Receive(api.Endpoints.FileCopyStop.Name, srv.FileCopyCancelHandle)

	err := srv.receiver.Start()
	if err == nil {
		defer srv.receiver.Stop()

		<-srv.stop // Stop signal waiting.
	}
	return err
}

// Stops the netfs server.
func (srv *Server) Stop() error {
	srv.stop <- syscall.SIGINT
	close(srv.stop)
	close(srv.copyScheduler.cancel) // TODO. cancel all active tasks
	return nil
}

// New instance of the netfs server.
func NewServer(config *ServerConfig) (*Server, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.Log.Level}))
	network, err := api.NewNetwork(config.Network)
	if err == nil {
		var receiver transport.TransportReceiver
		if receiver, err = transport.NewReceiver(config.Network.Protocol, config.Network.Port); err == nil {
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

			rootList := make([]api.FileInfo, len(config.RootList))
			for index, rootItem := range config.RootList {
				var osInfo os.FileInfo
				if osInfo, err = os.Stat(rootItem); err == nil {
					fileType := api.FILE
					if osInfo.IsDir() {
						fileType = api.DIRECTORY
					}

					rootList[index] = api.FileInfo{
						Id:       api.FileId(rootItem),
						Name:     osInfo.Name(),
						Path:     rootItem,
						Type:     fileType,
						Size:     api.FileSize(osInfo.Size()),
						ParentId: api.FileId(rootDirectory),
					}
				} else {
					break
				}
			}

			if err == nil {
				return &Server{
					log: log,
					copyScheduler: &CopyScheduler{
						log:     log,
						lock:    sync.Mutex{},
						tasks:   make([]*api.RemoteCopyTask, 100),
						network: network,
						cancel:  make(chan api.TaskId),
					},
					network:  network,
					receiver: receiver,
					rootList: rootList,
					stop:     stop,
				}, nil
			}
		}
	}
	return nil, err
}

// Stops the server by request from current host.
func (srv *Server) StopServerHandle(req transport.Request) ([]byte, any, error) { // TODO. Check current host.
	return nil, nil, srv.Stop()
}

// Returns information about the current host.
func (srv *Server) ServerHostHandle(req transport.Request) ([]byte, any, error) {
	return nil, srv.network.LocalHost(), nil
}

// The function handles request and returns information about the file.
func (srv *Server) FileInfoHandle(req transport.Request) ([]byte, any, error) {
	var info *api.FileInfo

	fileId, err := req.ParamRequired(api.Endpoints.FileInfo.FileId)
	if err == nil {
		srv.log.Info("FileInfoHandle()", "fileId", fileId)

		var osInfo os.FileInfo
		if osInfo, err = os.Stat(fileId); err == nil {
			fileType := api.FILE
			if osInfo.IsDir() {
				fileType = api.DIRECTORY
			}

			info = &api.FileInfo{
				Id:       api.FileId(fileId),
				Name:     osInfo.Name(),
				Path:     fileId,
				Type:     fileType,
				Size:     api.FileSize(osInfo.Size()),
				ParentId: api.FileId(filepath.Dir(fileId)),
			}
		}
	}

	if err != nil {
		srv.log.Error("FileInfoHandle()", "error", err)
		return nil, nil, err
	} else {
		srv.log.Info("FileInfoHandle()", "info", *info)
		return nil, info, nil
	}
}

// The function handles request and returns children of the directory.
func (srv *Server) FileChildrenHandle(req transport.Request) ([]byte, any, error) {
	var children []api.FileInfo

	fileId, err := req.ParamRequired(api.Endpoints.FileChildren.FileId)
	if err == nil {
		srv.log.Info("FileChildrenHandle()", "fileId", fileId)

		if fileId == rootDirectory {
			children = srv.rootList
		} else {
			var entries []fs.DirEntry
			if entries, err = os.ReadDir(fileId); err == nil {
				children = make([]api.FileInfo, len(entries))

				for index, entry := range entries {
					var osInfo fs.FileInfo
					if osInfo, err = entry.Info(); err == nil {
						fileType := api.FILE
						if osInfo.IsDir() {
							fileType = api.DIRECTORY
						}

						name := osInfo.Name()
						path := filepath.Join(fileId, name)
						children[index] = api.FileInfo{
							Id:       api.FileId(path),
							Name:     name,
							Path:     path,
							Type:     fileType,
							Size:     api.FileSize(osInfo.Size()),
							ParentId: api.FileId(fileId),
						}
					} else {
						break
					}
				}
			}
		}
	}

	if err != nil {
		srv.log.Error("FileChildrenHandle()", "error", err)
		return nil, nil, err
	} else {
		srv.log.Info("FileChildrenHandle()", "children", children)
		return nil, children, nil
	}
}

// The function handles request and creates a new file or directory by api.FileInfo.
func (srv *Server) FileCreateHandle(req transport.Request) ([]byte, any, error) {
	var err error

	replace := false
	replaceParam := req.Param(api.Endpoints.FileCreate.Replace)
	if replaceParam != "" {
		replace, err = strconv.ParseBool(replaceParam)
	}

	info := &api.FileInfo{}
	if err == nil {
		if _, err = req.Body(info); err == nil {
			srv.log.Info("FileCreateHandle()", "file", *info)

			if info.Path == "" || info.Type == 0 {
				err = errors.New("path and type are required fields")
			} else {
				if _, exists := os.Stat(info.Path); !replace && !errors.Is(exists, os.ErrNotExist) {
					err = ErrFileAlreadyExists
				} else {
					if info.Type == api.DIRECTORY {
						err = os.MkdirAll(info.Path, 0777)
					} else {
						parent := filepath.Dir(info.Path)
						if err = os.MkdirAll(parent, 0777); err == nil {
							if replace {
								os.Remove(info.Path)
							}

							var file *os.File
							if file, err = os.Create(info.Path); file != nil {
								file.Chmod(0777)
								file.Close()
							}
						}
					}
				}
			}
		}
	}

	if err != nil {
		srv.log.Error("FileCreateHandle()", "error", err)
		return nil, nil, err
	} else {
		srv.log.Info("FileCreateHandle()", "file", *info)

		info.Id = api.FileId(info.Path)
		info.Name = filepath.Base(info.Path)
		info.ParentId = api.FileId(filepath.Dir(info.Path))
		return nil, info, nil
	}
}

// The function handles request and writes data to a file.
func (srv *Server) FileWriteHandle(req transport.Request) ([]byte, any, error) {
	fileId, err := req.ParamRequired(api.Endpoints.FileInfo.FileId)
	if err == nil {
		data := req.RawBody()
		srv.log.Info("FileWriteHandle()", "fileId", "bytes", fileId, len(data))

		var file *os.File
		file, err = os.OpenFile(fileId, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
		if err == nil {
			file.Write(req.RawBody())
			err = file.Close()
		}
	}

	if err != nil {
		srv.log.Error("FileWriteHandle()", "error", err)
	}
	return nil, nil, err
}

// The function handles request and removes the file.
func (srv *Server) FileRemoveHandle(req transport.Request) ([]byte, any, error) {
	fileId, err := req.ParamRequired(api.Endpoints.FileInfo.FileId)
	if err == nil {
		srv.log.Info("FileRemoveHandle()", "fileId", fileId)
		err = os.RemoveAll(fileId)
	}

	if err != nil {
		srv.log.Error("FileRemoveHandle()", "error", err)
	}
	return nil, nil, err
}

// The function handles request and returns information about all tasks.
func (srv *Server) FileCopyHandle(req transport.Request) ([]byte, any, error) {
	tasks := srv.copyScheduler.Tasks()
	srv.log.Info("FileCopyHandle()", "tasks", tasks)

	return nil, tasks, nil
}

// The function handles request and starts a new task to copy the file or directory.
func (srv *Server) FileCopyStartHandle(req transport.Request) ([]byte, any, error) {
	task := &api.RemoteCopyTask{}

	_, err := req.Body(task)
	if err == nil {
		srv.log.Info("FileCopyStartHandle()", "task", task)

		target := &task.Target
		if target.Info.Type == api.FILE {
			err = target.Remove(srv.network.Transport())
		}

		if err == nil {
			if target, err = target.Host.Create(srv.network.Transport(), target.Info, true); err == nil {
				task.Target = *target
				err = srv.copyScheduler.StartTask(task)
			}
		}
	}

	if err != nil {
		srv.log.Error("FileCopyStartHandle()", "error", err)
	}
	return nil, task, err
}

// The function handles request and returns status of the task.
func (srv *Server) FileCopyStatusHandle(req transport.Request) ([]byte, any, error) { // TODO.
	return nil, nil, nil
}

// The function handles request and stops the task.
func (srv *Server) FileCopyCancelHandle(req transport.Request) ([]byte, any, error) {
	taskId, err := req.ParamRequired(api.Endpoints.FileCopyCancel.TaskId)
	if err == nil {
		srv.copyScheduler.CancelTask(api.TaskId(taskId))
	}

	if err != nil {
		srv.log.Error("FileCopyCancelHandle()", "error", err)
	}
	return nil, nil, err
}

type CopyScheduler struct {
	log     *slog.Logger
	lock    sync.Mutex
	tasks   []*api.RemoteCopyTask
	network *api.Network
	cancel  chan api.TaskId
}

func (sch *CopyScheduler) Tasks() []api.RemoteCopyTask {
	tasks := []api.RemoteCopyTask{}
	for _, task := range sch.tasks {
		if task != nil && (task.Status == api.Running || task.Status == api.Failed) {
			tasks = append(tasks, *task)
		}
	}
	return tasks
}

func (sch *CopyScheduler) StartTask(task *api.RemoteCopyTask) error {
	sch.lock.Lock()
	defer sch.lock.Unlock()

	// Check an empty position.
	taskIndex := -1
	for index := range sch.tasks {
		if sch.tasks[index] == nil {
			taskIndex = index
			break
		}
	}

	// Check the failed or completed task.
	if taskIndex == -1 {
		for index := range sch.tasks {
			status := sch.tasks[index].Status
			if status != api.Running {
				taskIndex = index
				break
			}
		}
	}

	if taskIndex != -1 {
		sch.tasks[taskIndex] = task

		if task.Source.Info.Type == api.FILE {
			task.Count = 1
			task.Current = 1

			go sch.copyFile(task, sch.cancel)
		} else {
			go sch.copyDirectory(task, sch.cancel)
		}
		return nil
	}
	return ErrTooManyActiveTasks
}

func (sch *CopyScheduler) CancelTask(taskId api.TaskId) {
	sch.cancel <- taskId
}

func (sch *CopyScheduler) copyDirectory(task *api.RemoteCopyTask, cancel chan api.TaskId) {
	sch.log.Info("CopyDirectory()", "taskId", task.Id, "started", true)

	source := &task.Source
	err := filepath.WalkDir(source.Info.Path, func(path string, entry fs.DirEntry, err error) error {
		if path != source.Info.Path {
			task.Count++
		}
		return err
	})

	sch.log.Info("CopyDirectory()", "taskId", task.Id, "count", task.Count)
	if err == nil && task.Count > 0 {
		task.Current = 1
		task.Status = api.Running

		target := &task.Target
		err = filepath.WalkDir(source.Info.Path, func(path string, entry fs.DirEntry, err error) error {
			if path != source.Info.Path {
				sch.log.Info("CopyDirectory()", "taskId", task.Id, "path", path)

				if err == nil && task.Status == api.Running {
					select {
					case taskId := <-cancel:
						if taskId == task.Id {
							task.Status = api.Cancelled
							sch.log.Info("CopyDirectory()", "taskId", taskId, "cancelled", true)
						}
					default:
						targetPath := strings.ReplaceAll(path, source.Info.Path, target.Info.Path)
						sch.log.Info("CopyDirectory()", "taskId", task.Id, "source", path, "target", targetPath)

						if entry.IsDir() {
							_, err = target.Host.Create(
								sch.network.Transport(),
								api.FileInfo{Id: api.FileId(targetPath), Name: entry.Name(), Type: api.DIRECTORY, Path: targetPath, ParentId: api.FileId(filepath.Dir(targetPath))},
								true,
							)
						} else {
							err = sch.copyFile(
								&api.RemoteCopyTask{
									Id:   task.Id,
									Host: task.Host,
									Source: api.RemoteFile{
										Host: source.Host,
										Info: api.FileInfo{Id: api.FileId(path), Name: entry.Name(), Type: api.FILE, Path: path, ParentId: api.FileId(filepath.Dir(path))},
									},
									Target: api.RemoteFile{
										Host: target.Host,
										Info: api.FileInfo{Id: api.FileId(targetPath), Name: entry.Name(), Type: api.FILE, Path: targetPath, ParentId: api.FileId(filepath.Dir(targetPath))},
									},
								},
								cancel,
							)
						}
					}

					if err == nil {
						if task.Current < task.Count {
							task.Progress = int(float32(task.Current) / float32(task.Count) * 100.0)
							task.Current++
							task.Status = api.Running
						} else {
							sch.log.Info("CopyDirectory()", "taskId", task.Id, "completed", true)
							task.Progress = 100
							task.Status = api.Completed
						}
					}
				}
			}
			return err
		})
	}

	if err != nil {
		task.Error = err
		task.Status = api.Failed

		sch.log.Error("CopyDirectory()", "error", err)
	}
}

func (sch *CopyScheduler) copyFile(task *api.RemoteCopyTask, cancel chan api.TaskId) error {
	sch.log.Info("CopyFile()", "taskId", task.Id, "started", true)

	source := &task.Source
	file, err := os.Open(source.Info.Path)
	if err == nil {
		var info os.FileInfo
		if info, err = file.Stat(); err == nil {
			task.Progress = 0
			task.Status = api.Running

			read := 0
			offset := int64(0)
			size := info.Size()
			buffer := make([]byte, min(size, 10485760)) // TODO. add pool

			client := sch.network.Transport()
			target := &task.Target
			if target, err = target.Host.Create(client, target.Info, true); err == nil {
				startTime := time.Now()
				progressPercent := float64(size) / 100.0
				for err == nil && task.Status == api.Running {
					select {
					case taskId := <-cancel:
						if taskId == task.Id {
							if err = target.Remove(client); err == nil {
								task.Status = api.Cancelled
								sch.log.Info("CopyFile()", "taskId", taskId, "cancelled", true)
							} else {
								sch.log.Info("CopyFile()", "taskId", taskId, "cancelled", false)
							}
						}
					default:
						if size > 0 {
							if read, err = file.ReadAt(buffer, offset); read > 0 {
								if err = target.Write(client, buffer[:read]); err == nil {
									offset += int64(read)
									task.Progress = int(min((float64(offset) / progressPercent), 100.0))
								}

								sch.log.Info("CopyFile()", "taskId", task.Id, "offset", offset, "progress", task.Progress)
							}
						}

						if size == 0 || errors.Is(err, io.EOF) {
							err = nil
							endTime := time.Now()
							task.Progress = 100.0
							task.Status = api.Completed
							sch.log.Info("CopyFile()", "taskId", task.Id, "progress", task.Progress, "duration", endTime.Sub(startTime), "completed", true)
						}
					}
				}
			}
		}
	}

	if file != nil {
		err = errors.Join(err, file.Close())
	}

	if err != nil {
		task.Error = err
		task.Status = api.Failed

		sch.log.Error("CopyFile()", "error", err)
	}
	return err
}

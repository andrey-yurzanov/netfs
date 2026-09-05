package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"netfs/api"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const rootDirectory = "/"
const defaultRoot = "./"
const defaultPort = 8989
const defaultTimeout = 2 * time.Second

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
		Network:  api.NetworkConfig{Port: defaultPort, Timeout: defaultTimeout},
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
	rootList  []api.FileInfo
	scheduler *CopyScheduler
	log       *slog.Logger
	network   *api.Network
	localhost *api.Host
	stop      chan os.Signal
}

func (srv *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/host", srv.handle(srv.Host))

	mux.HandleFunc("GET /api/file", srv.handle(srv.File))
	mux.HandleFunc("POST /api/file", srv.handle(srv.CreateFile))
	mux.HandleFunc("DELETE /api/file", srv.handle(srv.RemoveFile))
	mux.HandleFunc("POST /api/file/data", srv.handle(srv.WriteToFile))
	mux.HandleFunc("GET /api/file/children", srv.handle(srv.FileChildren))
	mux.HandleFunc("PUT /api/file/name", srv.handle(srv.RenameFile))

	mux.HandleFunc("GET /api/task/copy", srv.handle(srv.CopyFileTasks))
	mux.HandleFunc("POST /api/task/copy", srv.handle(srv.CopyFile))
	mux.HandleFunc("DELETE /api/task/copy", srv.handle(srv.CancelCopyFile))

	go http.ListenAndServe(":8989", mux)
	<-srv.stop // Stop signal waiting.

	return nil
}

func (srv *Server) Stop() error {
	srv.stop <- syscall.SIGINT
	close(srv.stop)

	srv.scheduler.CancelAll()
	close(srv.scheduler.cancel)

	return nil
}

func (srv *Server) Host(req *http.Request) (any, error) {
	return srv.localhost, nil
}

func (srv *Server) File(req *http.Request) (any, error) {
	fileId, err := srv.parseString("fileId", req)
	if err == nil {
		var info os.FileInfo
		if info, err = os.Stat(fileId); err == nil {
			fileType := api.FILE
			if info.IsDir() {
				fileType = api.DIRECTORY
			}

			fileId = filepath.ToSlash(fileId)
			return api.FileInfo{
				Id:       api.FileId(fileId),
				Name:     info.Name(),
				Path:     fileId,
				Type:     fileType,
				Size:     api.FileSize(info.Size()),
				ParentId: api.FileId(filepath.ToSlash(filepath.Dir(fileId))),
			}, nil
		} else {
			err = api.NewResponseError(api.ErrFileNotFound, err.Error())
		}
	}
	return nil, err
}

func (srv *Server) CreateFile(req *http.Request) (any, error) {
	replace, err := srv.parseBool("replace", req)
	if err == nil {
		file := api.FileInfo{}
		if _, err = api.Unmarshal(req.Body, &file); err == nil {
			if _, existsErr := os.Stat(file.Path); !replace && !errors.Is(existsErr, os.ErrNotExist) {
				err = api.NewResponseError(api.ErrFileAlreadyExists, "file [", file.Path, "] already exists")
			}
		}

		if err == nil {
			file.Path = filepath.ToSlash(file.Path)
			parent := filepath.ToSlash(filepath.Dir(file.Path))

			if file.Type == api.DIRECTORY {
				err = os.MkdirAll(file.Path, 0777) // TODO. to settings?
			} else {
				if err = os.MkdirAll(parent, 0777); err == nil {
					if replace {
						os.Remove(file.Path)
					}

					var created *os.File
					if created, err = os.Create(file.Path); err == nil {
						created.Chmod(0777)
						created.Close()
					}
				}
			}

			if err == nil {
				file.Id = api.FileId(file.Path)
				file.Name = filepath.Base(file.Path)
				file.ParentId = api.FileId(parent)

				return file, nil
			}
		}
	}
	return nil, err
}

func (srv *Server) RemoveFile(req *http.Request) (any, error) {
	fileId, err := srv.parseString("fileId", req)
	if err == nil {
		err = os.RemoveAll(fileId)
	}
	return nil, err
}

func (srv *Server) WriteToFile(req *http.Request) (any, error) {
	fileId, err := srv.parseString("fileId", req)
	if err == nil {
		var file *os.File
		file, err = os.OpenFile(fileId, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
		if err == nil {
			defer file.Close()

			_, err = io.Copy(file, req.Body)
		}
	}
	return nil, err
}

// BUG. Returns empty list, after copy a single file.
func (srv *Server) FileChildren(req *http.Request) (any, error) {
	fileId, err := srv.parseString("fileId", req)
	if err == nil {
		if fileId == rootDirectory {
			return srv.rootList, nil
		} else {
			if _, err = os.Stat(fileId); err != nil {
				return nil, api.NewResponseError(api.ErrFileNotFound, "file [", fileId, "] is not found")
			}

			var entries []fs.DirEntry
			if entries, err = os.ReadDir(fileId); err == nil {
				files := make([]api.FileInfo, len(entries))
				for index, entry := range entries {
					var info fs.FileInfo
					if info, err = entry.Info(); err == nil {
						fileType := api.FILE
						if info.IsDir() {
							fileType = api.DIRECTORY
						}

						name := info.Name()
						path := filepath.ToSlash(filepath.Join(fileId, name))
						files[index] = api.FileInfo{
							Id:       api.FileId(path),
							Name:     name,
							Path:     path,
							Type:     fileType,
							Size:     api.FileSize(info.Size()),
							ParentId: api.FileId(filepath.ToSlash(fileId)),
						}
					} else {
						break
					}
				}

				if err == nil {
					return files, nil
				}
			}
		}
	}
	return nil, err
}

func (srv *Server) RenameFile(req *http.Request) (any, error) {
	fileId, err := srv.parseString("fileId", req)
	if err == nil {
		name := ""
		if name, err = srv.parseString("name", req); err == nil {
			path := filepath.Join(filepath.Dir(fileId), name)
			err = os.Rename(fileId, path)
		}
	}
	return nil, err
}

func (srv *Server) CopyFile(req *http.Request) (any, error) {
	fileId, err := srv.parseString("fileId", req)
	if err == nil {
		fileId = filepath.ToSlash(fileId)

		var info os.FileInfo
		if info, err = os.Stat(fileId); err == nil {
			fileType := api.FILE
			if info.IsDir() {
				fileType = api.DIRECTORY
			}

			source := api.File{
				Host: srv.localhost,
				Info: api.FileInfo{
					Id:       api.FileId(fileId),
					Name:     info.Name(),
					Path:     fileId,
					Type:     fileType,
					Size:     api.FileSize(info.Size()),
					ParentId: api.FileId(filepath.ToSlash(filepath.Dir(fileId))),
				},
			}

			target := &api.File{}
			if target, err = api.Unmarshal(req.Body, target); err == nil {
				target.Host.Network = api.NewNetwork(target.Host.Network.Config) // TODO. Remove after refactoring.

				taskId := filepath.ToSlash(filepath.Join(target.Host.IP.String(), string(target.Info.Id)))
				task := api.CopyTask{Id: api.TaskId(taskId), Source: source, Target: *target, Host: srv.localhost}
				if err = srv.scheduler.Start(&task); err == nil {
					return task, nil
				}
			}

		}
	}
	return nil, err
}

func (srv *Server) CopyFileTasks(req *http.Request) (any, error) {
	return srv.scheduler.Tasks(), nil
}

func (srv *Server) CancelCopyFile(req *http.Request) (any, error) {
	taskId, err := srv.parseString("taskId", req)
	if err == nil {
		srv.scheduler.Cancel(api.TaskId(taskId))
	}
	return nil, err
}

func (srv *Server) parseBool(name string, req *http.Request) (bool, error) {
	value := req.URL.Query().Get(name)
	if value != "" {
		return strconv.ParseBool(value)
	}
	return false, api.NewResponseError(api.ErrParameterIsRequired, "parameter [", name, "] is required")
}

func (srv *Server) parseString(name string, req *http.Request) (string, error) {
	value := req.URL.Query().Get(name)
	if value != "" {
		return value, nil
	}
	return "", api.NewResponseError(api.ErrParameterIsRequired, "parameter [", name, "] is required")
}

func (srv *Server) handle(handler func(req *http.Request) (any, error)) func(wrt http.ResponseWriter, res *http.Request) {
	return func(wrt http.ResponseWriter, req *http.Request) {
		srv.log.Debug("handle()", "url", req.URL, "method", req.Method)

		var res []byte
		var err error
		var status int = http.StatusOK

		var data any
		if data, err = handler(req); err == nil && data != nil {
			res, err = json.Marshal(data)
		}

		if err != nil {
			if resErr, ok := err.(*api.ResponseError); ok {
				status = resErr.Code.HttpStatusCode()
				res, err = json.Marshal(resErr)
			} else {
				status = http.StatusInternalServerError
			}
		}

		wrt.Header().Add(api.ContentType, api.JsonContentType)
		wrt.WriteHeader(status)
		wrt.Write(res)

		srv.log.Debug("handle()", "status", status, "response", string(res), "error", err)
	}
}

func NewServer(config *ServerConfig) (*Server, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.Log.Level}))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	network := api.NewNetwork(config.Network)
	host, err := network.LocalHost()
	if err == nil {
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
				log:       log,
				network:   network,
				localhost: host,
				rootList:  rootList,
				stop:      stop,
				scheduler: &CopyScheduler{
					log:     log,
					lock:    sync.Mutex{},
					tasks:   make([]*api.CopyTask, 100), // TODO. from settings
					network: network,
					cancel:  make(chan api.TaskId),
				},
			}, nil
		}
	}
	return nil, err
}

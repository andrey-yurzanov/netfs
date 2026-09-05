package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

var units = [5]string{"B", "KB", "MB", "GB", "TB"}

// Type of file.
type FileType byte

const (
	FILE FileType = 1 << iota
	DIRECTORY
)

// Returns a string representation of the file type.
func (fileType FileType) String() string {
	if fileType == FILE {
		return "f"
	}
	return "d"
}

// Size of the file.
type FileSize int64

// String representation of file size.
func (fileSize FileSize) String() string {
	size := int64(fileSize)
	if size == 0 {
		return "0"
	} else {
		unit := 0
		for unit < len(units) && size >= 1024 {
			size /= 1024
			unit++
		}
		return strings.Join(
			[]string{
				strconv.FormatInt(size, decimalBase),
				units[unit],
			},
			" ",
		)
	}
}

// File identifier.
type FileId string

// Information about file.
type FileInfo struct {
	Id       FileId
	Name     string
	Path     string
	Type     FileType
	Size     FileSize
	ParentId FileId
}

// File on a remote host.
type File struct {
	Info FileInfo
	Host *Host
}

func (file *File) Children() ([]File, error) {
	client := file.Host.Network.client
	url := BuildUrl(file.Host.IP, file.Host.Network.Config.Port, "/api/file/children", "fileId", string(file.Info.Id))
	res, err := client.Get(url)
	if err == nil {
		defer res.Body.Close()

		if res.StatusCode == http.StatusOK {
			files := []FileInfo{}
			files, err = UnmarshalArray(res.Body, &files)

			result := make([]File, len(files))
			for index, info := range files {
				result[index] = File{Info: info, Host: file.Host}
			}
			return result, nil
		} else {
			err = unmarshalError(res.Body)
		}
	}
	return nil, err
}

func (file *File) Write(data []byte) error {
	client := file.Host.Network.client
	url := BuildUrl(file.Host.IP, file.Host.Network.Config.Port, "/api/file/data", "fileId", string(file.Info.Id))
	res, err := client.Post(url, BinaryContentType, bytes.NewReader(data))
	if err == nil {
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			err = unmarshalError(res.Body)
		}
	}
	return err
}

func (file *File) CopyTo(target File) (*CopyTask, error) {
	data, err := json.Marshal(target)
	if err == nil {
		client := file.Host.Network.client
		url := BuildUrl(file.Host.IP, file.Host.Network.Config.Port, "/api/task/copy", "fileId", string(file.Info.Id))

		var res *http.Response
		if res, err = client.Post(url, JsonContentType, bytes.NewReader(data)); err == nil {
			defer res.Body.Close()

			if res.StatusCode == http.StatusOK {
				return Unmarshal(res.Body, &CopyTask{Host: file.Host})
			} else {
				err = unmarshalError(res.Body)
			}
		}
	}
	return nil, err
}

func (file *File) Remove() error {
	client := file.Host.Network.client
	url := BuildUrl(file.Host.IP, file.Host.Network.Config.Port, "/api/file", "fileId", string(file.Info.Id))

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err == nil {
		var res *http.Response
		if res, err = client.Do(req); err == nil {
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				err = unmarshalError(res.Body)
			}
		}
	}
	return err
}

func (file *File) Rename(name string) error {
	client := file.Host.Network.client
	url := BuildUrl(file.Host.IP, file.Host.Network.Config.Port, "/api/file/name", "fileId", string(file.Info.Id), "name", name)

	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err == nil {
		var res *http.Response
		if res, err = client.Do(req); err == nil {
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				err = unmarshalError(res.Body)
			}
		}
	}
	return err
}

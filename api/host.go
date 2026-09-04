package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
)

const rootDirectory = "/"

type Host struct {
	Name    string
	IP      net.IP
	Network *Network
}

func (host *Host) Root() *File {
	return &File{Host: host, Info: FileInfo{Id: rootDirectory, Path: rootDirectory}}
}

func (host *Host) Create(file FileInfo, replace bool) (*File, error) {
	data, err := json.Marshal(file)
	if err == nil {
		client := host.Network.client
		url := BuildUrl(host.IP, host.Network.Config.Port, "/api/file", "replace", strconv.FormatBool(replace))

		var res *http.Response
		res, err = client.Post(url, JsonContentType, bytes.NewReader(data))
		if err == nil {
			defer res.Body.Close()

			if res.StatusCode == http.StatusOK {
				info := &FileInfo{}
				if info, err = Unmarshal(res.Body, info); err == nil {
					return &File{Info: *info, Host: host}, nil
				}
			} else {
				err = unmarshalError(res.Body)
			}
		}
	}
	return nil, err
}

func (host *Host) File(fileId FileId) (*File, error) {
	url := BuildUrl(host.IP, host.Network.Config.Port, "/api/file", "fileId", string(fileId))
	client := host.Network.client
	res, err := client.Get(url)
	if err == nil {
		defer res.Body.Close()

		if res.StatusCode == http.StatusOK {
			return Unmarshal(res.Body, &File{Host: host})
		} else {
			err = unmarshalError(res.Body)
		}
	}
	return nil, err
}

func (host *Host) Tasks() ([]CopyTask, error) {
	client := host.Network.client
	res, err := client.Get(BuildUrl(host.IP, host.Network.Config.Port, "/api/task/copy"))
	if err == nil {
		defer res.Body.Close()

		if res.StatusCode == http.StatusOK {
			tasks := []CopyTask{}
			if tasks, err = UnmarshalArray(res.Body, &tasks); err == nil {
				for index, _ := range tasks {
					task := tasks[index]
					task.Host = host
				}
				return tasks, nil
			}
		} else {
			err = unmarshalError(res.Body)
		}
	}
	return nil, err
}

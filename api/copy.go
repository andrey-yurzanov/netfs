package api

import "net/http"

// Status of the task.
type TaskStatus uint8

const (
	Failed TaskStatus = iota
	Running
	Cancelled
	Completed
)

// The task identifier.
type TaskId string

// Netfs server task.
type CopyTask struct {
	Source   File
	Target   File
	Id       TaskId
	Error    error
	Progress int
	Count    int
	Current  int
	Status   TaskStatus
	Host     *Host
}

func (task *CopyTask) Cancel() error {
	client := task.Host.Network.client
	url := BuildUrl(task.Host.IP, task.Host.Network.Config.Port, "/api/task", "taskId", string(task.Id))
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

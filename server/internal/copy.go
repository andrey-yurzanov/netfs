package server

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"netfs/api"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CopyScheduler struct {
	log     *slog.Logger
	lock    sync.Mutex
	tasks   []*api.CopyTask
	network *api.Network
	cancel  chan api.TaskId
}

func (sch *CopyScheduler) Tasks() []api.CopyTask {
	tasks := []api.CopyTask{}
	for _, task := range sch.tasks {
		if task != nil {
			tasks = append(tasks, *task)
		}
	}
	return tasks
}

func (sch *CopyScheduler) Start(task *api.CopyTask) error {
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

func (sch *CopyScheduler) Cancel(taskId api.TaskId) {
	sch.cancel <- taskId
}

func (sch *CopyScheduler) CancelAll() {
	sch.lock.Lock()
	defer sch.lock.Unlock()

	for _, task := range sch.tasks {
		sch.Cancel(task.Id)
	}
}

func (sch *CopyScheduler) copyDirectory(task *api.CopyTask, cancel chan api.TaskId) {
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
		host := target.Host
		err = filepath.WalkDir(source.Info.Path, func(path string, entry fs.DirEntry, err error) error {
			if path != source.Info.Path {
				path = filepath.ToSlash(path)
				sch.log.Info("CopyDirectory()", "taskId", task.Id, "path", path)

				if err == nil && task.Status == api.Running {
					select {
					case taskId := <-cancel:
						if taskId == task.Id {
							task.Status = api.Cancelled
							sch.log.Info("CopyDirectory()", "taskId", taskId, "cancelled", true)
							return filepath.SkipAll
						}
					default:
						targetPath := strings.ReplaceAll(path, source.Info.Path, target.Info.Path)
						sch.log.Info("CopyDirectory()", "taskId", task.Id, "source", path, "target", targetPath)

						if entry.IsDir() {
							_, err = host.Create(
								api.FileInfo{
									Id:       api.FileId(targetPath),
									Name:     entry.Name(),
									Type:     api.DIRECTORY,
									Path:     targetPath,
									ParentId: api.FileId(filepath.Dir(targetPath)),
								},
								true,
							)
						} else {
							childTask := &api.CopyTask{
								Id:   task.Id,
								Host: task.Host,
								Source: api.File{
									Host: source.Host,
									Info: api.FileInfo{
										Id:       api.FileId(path),
										Name:     entry.Name(),
										Type:     api.FILE,
										Path:     path,
										ParentId: api.FileId(filepath.Dir(path)),
									},
								},
								Target: api.File{
									Host: target.Host,
									Info: api.FileInfo{
										Id:       api.FileId(targetPath),
										Name:     entry.Name(),
										Type:     api.FILE,
										Path:     targetPath,
										ParentId: api.FileId(filepath.Dir(targetPath)),
									},
								},
							}

							err = sch.copyFile(childTask, cancel)
							if childTask.Status == api.Cancelled {
								task.Status = api.Cancelled
								sch.log.Info("CopyDirectory()", "taskId", task.Id, "cancelled", true)
								return filepath.SkipAll
							}
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

func (sch *CopyScheduler) copyFile(task *api.CopyTask, cancel chan api.TaskId) error {
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

			target := &task.Target
			if target, err = target.Host.Create(target.Info, true); err == nil {
				startTime := time.Now()
				progressPercent := float64(size) / 100.0
				for err == nil && task.Status == api.Running {
					select {
					case taskId := <-cancel:
						if taskId == task.Id {
							if err = target.Remove(); err == nil {
								task.Status = api.Cancelled
								sch.log.Info("CopyFile()", "taskId", taskId, "cancelled", true)
							} else {
								sch.log.Info("CopyFile()", "taskId", taskId, "cancelled", false)
							}
						}
					default:
						if size > 0 {
							if read, err = file.ReadAt(buffer, offset); read > 0 {
								if err = target.Write(buffer[:read]); err == nil {
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

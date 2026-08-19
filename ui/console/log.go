package console

import "os"

func Log(path string, message string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
	if err != nil {
		panic(err)
	}

	file.WriteString(message)
	file.Close()
}

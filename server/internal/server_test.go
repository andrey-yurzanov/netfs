package server_test

import (
	server "netfs/server/internal"
	"os"
	"strconv"
	"testing"
)

func TestGenerateDirs(t *testing.T) {
	for index := range 100000 {
		os.MkdirAll("d:/application/test/test_"+strconv.Itoa(index), 0777)
	}
}

func TestRun(t *testing.T) {
	var config *server.ServerConfig
	var err error

	if len(os.Args) > 1 {
		config, err = server.ReadServerConfig(os.Args[1])
	}

	if config == nil {
		config, err = server.ReadServerConfig(server.DefaultConfigPath)
	}

	// Default configuration.
	if config == nil {
		config, err = server.WriteServerConfig(server.NewServerConfig())
	}

	var srv *server.Server
	if srv, err = server.NewServer(config); err != nil {
		panic(err)
	}

	if err = srv.Start(); err != nil {
		panic(err)
	}
}

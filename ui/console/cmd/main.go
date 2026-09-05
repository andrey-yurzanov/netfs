package main

// import (
// 	"fmt"
// 	"netfs/api"
// 	"netfs/api/transport"
// 	"time"
// )

// func main() {
// 	network, err := api.NewNetwork(api.NetworkConfig{Port: 8989, Protocol: transport.HTTP, Timeout: time.Second * 5})
// 	if err != nil {
// 		panic(err)
// 	}

// 	hosts, err := network.Hosts()
// 	fmt.Println(hosts)
// }

import (
	"netfs/api"
	"netfs/ui/console"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	network := api.NewNetwork(api.NetworkConfig{Port: 8989, Timeout: time.Second * 1})
	program := tea.NewProgram(console.NewConsoleViewModel(network), tea.WithAltScreen())

	_, err := program.Run()
	if err != nil {
		panic(err)
	}
}

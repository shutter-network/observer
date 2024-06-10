package main

import (
	"fmt"
	"os"

	"github.com/shutter-network/gnosh-metrics/cmd/cli"
)

func main() {
	status := 0
	if err := cli.Cmd().Execute(); err != nil {
		fmt.Println("failed running server")
		status = 1
	}
	os.Exit(status)
}

package main

import (
	"fmt"
	"os"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/infrastructure/configfile"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

var version = "dev"

func main() {
	initializer := bootstrap.NewService(
		configfile.NewLoader(),
		storage.NewSQLiteInitializer(),
	)
	root := cli.NewRootCommand(initializer, version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

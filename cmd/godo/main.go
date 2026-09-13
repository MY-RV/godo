package main

import (
	"fmt"
	"os"

	"github.com/my-rv/godo"
	"github.com/my-rv/godo/internal/cli"
)

func main() {
	if err := cli.New().Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "godo: %v\n", err)
		os.Exit(godo.ExitCode(err))
	}
}

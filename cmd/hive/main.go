package main

import (
	"fmt"
	"os"

	"github.com/sakakibara/hive/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "hive: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"os"

	"symbol-output/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], bannerFiles, os.Stdout, os.Stderr))
}

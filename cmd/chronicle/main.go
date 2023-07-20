package main

import (
	"fmt"
	"os"

	"github.com/gookit/color"

	"github.com/anchore/chronicle/cmd/chronicle/cli"
)

func main() {
	root := cli.New()

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, color.Red.Sprint(err.Error()))
		os.Exit(1)
	}
}

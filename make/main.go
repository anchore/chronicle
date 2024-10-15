package main

import (
	. "github.com/anchore/go-make"
	. "github.com/anchore/go-make/tasks"
)

func main() {
	Makefile(
		LintFix,
		Format,
		Task{
			Name: "pwd",
			Desc: "where am i?",
			Run: func() {
				Run("pwd")
			},
		},
	)
}

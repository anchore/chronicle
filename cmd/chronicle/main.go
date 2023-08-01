package main

import (
	"github.com/anchore/chronicle/cmd/chronicle/cli"
	"github.com/anchore/chronicle/internal"
	"github.com/anchore/clio"
)

const valueNotProvided = "[not provided]"

// all variables here are provided as build-time arguments, with clear default values
var version = valueNotProvided
var buildDate = valueNotProvided
var gitCommit = valueNotProvided
var gitDescription = valueNotProvided

func main() {
	app := cli.New(
		clio.Identification{
			Name:           internal.ApplicationName,
			Version:        version,
			BuildDate:      buildDate,
			GitCommit:      gitCommit,
			GitDescription: gitDescription,
		},
	)

	app.Run()
}

package cliby

import (
	"gopkg.in/alecthomas/kingpin.v2"
)

type Interface interface {
	// SetCookieFile(string) *Cli
	// GetCookieFile(string) *Cli
	Name() string
	GetDefaults() interface{}
	NewOptions() interface{}
	GetOptions() interface{}
	SetOptions(interface{})
	CommandLine() *kingpin.Application
	SetCommands(map[string]func() error)
	GetCommand(string) func() error
}

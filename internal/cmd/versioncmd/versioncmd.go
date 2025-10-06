package versioncmd

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/peterbourgon/ff/v4"

	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
)

type Config struct {
	*rootcmd.RootConfig
	Command *ff.Command
	Flags   *ff.FlagSet
}

func New(parent *rootcmd.RootConfig) *Config {
	cfg := &Config{RootConfig: parent}
	cfg.Flags = ff.NewFlagSet("version").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "version",
		Usage:     "migrate version [FLAGS]",
		ShortHelp: "Print the current version of migrate.",
		Flags:     cfg.Flags,
		Exec:      cfg.Exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return cfg
}

func (cfg *Config) Exec(ctx context.Context, _ []string) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("build info not available")
	}

	_, err := fmt.Fprintf(cfg.Stdout, "migrate %s (built with %s)\n", info.Main.Version, info.GoVersion)

	return err
}

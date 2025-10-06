package exportcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	cfg.Flags = ff.NewFlagSet("export").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "export",
		Usage:     "migrate export <TYPE>",
		ShortHelp: "Export reports about the migrate workflows.",
		Flags:     cfg.Flags,
		Exec:      cfg.Exec,
	}

	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return cfg
}

func (cfg *Config) Exec(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("missing export type (move|replicate)")
	}

	app, err := cfg.App(ctx)
	if err != nil {
		return err
	}

	switch strings.ToLower(args[0]) {
	case "move":
		if err := app.ExportMove(ctx); err != nil {
			return fmt.Errorf("export move report: %w", err)
		}
	case "replicate":
		if err := app.ExportReplication(ctx); err != nil {
			return fmt.Errorf("export replication report: %w", err)
		}
	default:
		return fmt.Errorf("unsupported export type: %s", args[0])
	}

	return nil
}

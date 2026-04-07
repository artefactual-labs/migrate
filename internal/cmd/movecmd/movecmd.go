package movecmd

import (
	"context"
	"log/slog"

	"github.com/peterbourgon/ff/v4"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
)

type Config struct {
	*rootcmd.RootConfig
	Command *ff.Command
	Flags   *ff.FlagSet
}

func New(parent *rootcmd.RootConfig) *Config {
	cfg := &Config{RootConfig: parent}
	cfg.Flags = ff.NewFlagSet("move").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "move",
		Usage:     "migrate move",
		ShortHelp: "Move AIPs listed in input.txt via Temporal workflows.",
		Flags:     cfg.Flags,
		Exec:      cfg.Exec,
	}

	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return cfg
}

func (cfg *Config) Exec(ctx context.Context, _ []string) error {
	app, err := cfg.App(ctx)
	if err != nil {
		return err
	}

	uuids, err := application.LoadInputUUIDs()
	if err != nil {
		return err
	}

	slog.SetDefault(cfg.Logger())

	return application.RunWorkflowFromUUIDs(ctx, app, uuids)
}

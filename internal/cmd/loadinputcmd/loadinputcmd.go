package loadinputcmd

import (
	"context"
	"fmt"

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
	cfg.Flags = ff.NewFlagSet("load-input").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "load-input",
		Usage:     "migrate load-input",
		ShortHelp: "Populate the database with input UUIDs and export replication data.",
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

	for _, id := range uuids {
		if _, err := app.InitAIPInDatabase(ctx, id); err != nil {
			return fmt.Errorf("init AIP in database: %w", err)
		}
		if _, err := app.FindA(ctx, application.FindParams{AipID: id.String()}); err != nil {
			return fmt.Errorf("find AIP: %w", err)
		}
	}

	if err := app.ExportReplication(ctx); err != nil {
		return fmt.Errorf("export replication: %w", err)
	}

	return nil
}

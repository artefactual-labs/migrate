package servercmd

import (
	"context"
	"os"

	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
	"github.com/artefactual-labs/migrate/internal/web"
	"github.com/labstack/echo/v5"
	"github.com/peterbourgon/ff/v4"
)

type Config struct {
	*rootcmd.RootConfig
	Command *ff.Command
	Flags   *ff.FlagSet
}

func New(parent *rootcmd.RootConfig) *Config {
	cfg := &Config{RootConfig: parent}
	cfg.Flags = ff.NewFlagSet("server").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "server",
		Usage:     "migrate server",
		ShortHelp: "Run as a Web application",
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

	e := echo.New()
	e.Logger = cfg.Logger()

	endpoints := web.NewEndpoints(app)
	web.Routes(e, endpoints)

	// TODO: make this configurable
	addr := ":4001"
	if err := e.Start(addr); err != nil {
		e.Logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}
	return nil
}

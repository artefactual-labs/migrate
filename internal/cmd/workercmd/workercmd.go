package workercmd

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

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
	cfg.Flags = ff.NewFlagSet("worker").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "worker",
		Usage:     "migrate worker",
		ShortHelp: "Run the migrate Temporal worker until interrupted.",
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

	if err := runWorker(app); err != nil {
		return fmt.Errorf("run worker: %w", err)
	}

	return nil
}

func runWorker(app *application.App) error {
	w := registerWorker(app)
	return w.Run(worker.InterruptCh())
}

func registerWorker(app *application.App) worker.Worker {
	w := worker.New(app.Tc, app.Config.Temporal.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(
		application.NewReplicateWorkflow(app).Run,
		workflow.RegisterOptions{
			Name: application.ReplicateWorkflowName,
		},
	)

	w.RegisterWorkflowWithOptions(
		application.NewMoveWorkflow(app).Run,
		workflow.RegisterOptions{
			Name: application.MoveWorkflowName,
		},
	)

	w.RegisterActivityWithOptions(
		application.NewCheckStorageServiceConnectionActivity(app.StorageClient).Execute,
		activity.RegisterOptions{Name: application.CheckStorageServiceConnectionActivityName},
	)
	w.RegisterActivityWithOptions(app.InitAIPInDatabase, activity.RegisterOptions{Name: application.InitAIPInDatabaseName})
	w.RegisterActivityWithOptions(app.ReplicateA, activity.RegisterOptions{Name: application.ReplicateAName})
	w.RegisterActivityWithOptions(app.FindA, activity.RegisterOptions{Name: application.FindAName})
	w.RegisterActivityWithOptions(app.CheckReplicationStatus, activity.RegisterOptions{Name: application.CheckReplicationStatusName})
	w.RegisterActivityWithOptions(app.MoveA, activity.RegisterOptions{Name: application.MoveActivityName})

	return w
}

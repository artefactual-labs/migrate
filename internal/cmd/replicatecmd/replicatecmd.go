package replicatecmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

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
	cfg.Flags = ff.NewFlagSet("replicate").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "replicate",
		Usage:     "migrate replicate",
		ShortHelp: "Replicate AIPs listed in input.txt via Temporal workflows.",
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

	logger := cfg.Logger()
	logger.Info("Starting Replication")
	for _, l := range app.Locations.ReplicationTargets {
		logger.Info(fmt.Sprintf("Location Name %s, ID: %s", l.Name, l.ID))
	}

	for _, id := range uuids {
		workflowID := fmt.Sprintf("AIP_Replicate_%s", id.String())
		options := client.StartWorkflowOptions{
			ID:                    workflowID,
			TaskQueue:             app.Config.Temporal.TaskQueue,
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		}
		params := application.ReplicateWorkflowParams{
			UUID: id,
		}
		aip, err := app.GetAIPByID(ctx, id.String())
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get AIP by ID: %w", err)
		} else if aip != nil && aip.Status == string(application.AIPStatusReplicated) {
			logger.Info("AIP Already Replicated")
			continue
		} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
			logger.Info("AIP Not Found")
			continue
		}

		we, err := app.Tc.ExecuteWorkflow(ctx, options, application.ReplicateWorkflowName, params)
		if err != nil {
			logger.Error("workflow launch failed", "err", err)
			continue
		}
		var result application.ReplicateWorkflowResult
		err = we.Get(ctx, &result)
		if err != nil {
			logger.Error("workflow execution failed", "error", err)
			continue
		}
		logger.Info("workflow", "ID", we.GetID())
	}

	return nil
}

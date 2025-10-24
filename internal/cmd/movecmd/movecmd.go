package movecmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v4"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
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

	logger := cfg.Logger()

	for _, id := range uuids {
		workflowID := fmt.Sprintf("AIP_Move_%s", id.String())
		// Allow duplicate execution only when the previous run closed
		// unsuccessfully. This prevents two healthy runs from processing the
		// same AIP at the same time.
		options := client.StartWorkflowOptions{
			ID:                    workflowID,
			TaskQueue:             app.Config.Temporal.TaskQueue,
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		}
		params := application.MoveWorkflowParams{
			UUID: id,
		}
		aip, err := app.GetAIPByID(ctx, id.String())
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get AIP by ID: %w", err)
		} else if aip != nil && aip.Status == string(application.AIPStatusMoved) {
			logger.Info("AIP Already Moved")
			continue
		} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
			logger.Info("AIP Not Found")
			continue
		}

		var we client.WorkflowRun
		for {
			we, err = app.Tc.ExecuteWorkflow(ctx, options, application.MoveWorkflowName, params)
			if err != nil {
				var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
				if errors.As(err, &alreadyStarted) {
					logger.Info("Workflow already running, retrying shortly.", "workflow_id", workflowID)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(10 * time.Second):
					}
					continue
				}
				logger.Error("Workflow launch failed.", "err", err)
				break
			}
			break
		}
		if we == nil {
			continue
		}
		var result application.MoveWorkflowResult
		err = we.Get(ctx, &result)
		if err != nil {
			logger.Error("Workflow execution failed.", "error", err)
			continue
		}
		logger.Info("workflow", "ID", we.GetID())
	}

	return nil
}

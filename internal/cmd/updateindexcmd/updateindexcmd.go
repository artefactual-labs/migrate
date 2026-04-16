package updateindexcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/peterbourgon/ff/v4"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
)

type Config struct {
	*rootcmd.RootConfig
	Command             *ff.Command
	Flags               *ff.FlagSet
	DryRun              bool
	AllInTargetLocation bool
}

type summary struct {
	total       int
	updated     int
	wouldUpdate int
	noChange    int
	skipped     int
	failed      int
}

func New(parent *rootcmd.RootConfig) *Config {
	cfg := &Config{RootConfig: parent}
	cfg.Flags = ff.NewFlagSet("update-index").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.DryRun, 'n', "dry-run", "show what would change in Elasticsearch without updating documents")
	cfg.Flags.BoolVar(&cfg.AllInTargetLocation, 0, "all-in-target-location", "discover AIP UUIDs from the configured move target location instead of input.txt")

	cfg.Command = &ff.Command{
		Name:      "update-index",
		Usage:     "migrate update-index [--dry-run] [--all-in-target-location]",
		ShortHelp: "Update Elasticsearch AIP documents from input.txt or the configured move target location.",
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

	uuids, err := cfg.loadUUIDs(ctx, app)
	if err != nil {
		return err
	}

	logger := cfg.Logger()
	stats := summary{total: len(uuids)}

	for _, id := range uuids {
		workflowID := fmt.Sprintf("AIP_UpdateIndex_%s", id.String())
		options := client.StartWorkflowOptions{
			ID:                    workflowID,
			TaskQueue:             app.Config.Temporal.TaskQueue,
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		}
		params := application.UpdateIndexWorkflowParams{
			UUID:   id,
			DryRun: cfg.DryRun,
		}

		var we client.WorkflowRun
		for {
			we, err = app.Tc.ExecuteWorkflow(ctx, options, application.UpdateIndexWorkflowName, params)
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
				stats.failed++
				break
			}
			break
		}
		if we == nil {
			continue
		}

		var result application.UpdateIndexWorkflowResult
		err = we.Get(ctx, &result)
		if err != nil {
			logger.Error("Workflow execution failed.", "error", err)
			stats.failed++
			continue
		}
		stats.recordResult(result.Message)
		logger.Info("Workflow completed.", "id", we.GetID(), "result", result.Message)
	}

	args := []any{
		"total", stats.total,
	}
	if !cfg.DryRun || stats.updated > 0 {
		args = append(args, "updated", stats.updated)
	}
	args = append(args,
		"would_update", stats.wouldUpdate,
		"no_change", stats.noChange,
		"skipped", stats.skipped,
	)
	if !cfg.DryRun || stats.failed > 0 {
		args = append(args, "failed", stats.failed)
	}
	logger.Info("Update-index summary.", args...)

	return nil
}

func (cfg *Config) loadUUIDs(ctx context.Context, app *application.App) ([]uuid.UUID, error) {
	if !cfg.AllInTargetLocation {
		return application.LoadInputUUIDs()
	}

	packages, err := app.StorageClient.Packages.ListByLocation(ctx, app.Config.StorageService.Locations.MoveTargetLocationID)
	if err != nil {
		return nil, fmt.Errorf("list packages in target location: %w", err)
	}

	uuids := make([]uuid.UUID, 0, len(packages))
	for _, pkg := range packages {
		id, err := uuid.Parse(pkg.UUID)
		if err != nil {
			return nil, fmt.Errorf("parse package uuid %q: %w", pkg.UUID, err)
		}
		uuids = append(uuids, id)
	}

	return uuids, nil
}

func (s *summary) recordResult(message string) {
	switch {
	case strings.HasPrefix(message, "Updated Elasticsearch fields:"):
		s.updated++
	case strings.HasPrefix(message, "Dry run: would update Elasticsearch fields:"):
		s.wouldUpdate++
	case strings.HasPrefix(message, "Elasticsearch update not needed:"):
		s.noChange++
	case strings.HasPrefix(message, "Elasticsearch update skipped:"):
		s.skipped++
	default:
		s.failed++
	}
}

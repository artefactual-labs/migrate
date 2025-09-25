package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/stephenafamo/bob"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/durationpb"
	_ "modernc.org/sqlite"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/migrations"
)

func main() {
	var (
		ctx    = context.Background()
		args   = os.Args[1:]
		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr
	)

	if err := exec(ctx, args, stdin, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err) //nolint:errcheck
		os.Exit(1)
	}
}

func exec(ctx context.Context, args []string, _ io.Reader, _, stderr io.Writer) (err error) {
	loggerOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	logger := slog.New(slog.NewTextHandler(stderr, loggerOpts))
	slog.SetDefault(logger) // TODO: avoid global state.

	db, err := initDatabase(ctx, "migrate.db")
	if err != nil {
		return err
	}

	app := &application.App{}
	app.DB = db

	if cfgFile, err := os.ReadFile("config.json"); err != nil {
		return err
	} else if err := json.Unmarshal(cfgFile, &app.Config); err != nil {
		return fmt.Errorf("unmarshal config.json: %v", err)
	}

	// Connect with Temporal Server.
	// TODO(daniel): make all these options configurable.
	// TODO: push namespace registration to deployment.
	const temporalNamespace = "move"
	if tc, err := client.Dial(client.Options{
		Namespace: temporalNamespace,
		Logger:    logger,
	}); err != nil {
		return fmt.Errorf("dial temporal: %v", err)
	} else if nsc, err := client.NewNamespaceClient(client.Options{Namespace: temporalNamespace}); err != nil {
		return fmt.Errorf("new namespace client: %v", err)
	} else if err := nsc.Register(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace: temporalNamespace,
		WorkflowExecutionRetentionPeriod: &durationpb.Duration{
			Seconds: 31_536_000, /* 365 days. */
		},
	}); err != nil {
		return fmt.Errorf("register namespace: %v", err)
	} else {
		app.Tc = tc
		if err := StartWorker(app); err != nil {
			return fmt.Errorf("start worker: %v", err)
		}
	}

	var input []string
	f, err := os.Open("input.txt")
	if err != nil {
		return fmt.Errorf("open input.txt: %v", err)
	}
	s := bufio.NewScanner(f)
	for s.Scan() {
		input = append(input, s.Text())
	}

	UUIDs, err := application.ValidateUUIDs(input)
	if err != nil {
		return fmt.Errorf("validate UUIDs: %v", err)
	}

	if len(args) <= 1 {
		return errors.New("missing command")
	}
	command := args[1]

	switch command {
	case "worker":
		err := RunWorker(app)
		if err != nil {
			return fmt.Errorf("run worker: %v", err)
		}
	case "replicate":
		slog.Info("Starting Replication")
		for _, l := range app.Config.ReplicationLocations {
			slog.Info(fmt.Sprintf("Location Name %s, UUID: %s", l.Name, l.UUID))
		}

		for _, id := range UUIDs {
			WorkflowID := fmt.Sprintf("AIP_Replicate_%s", id.String())
			options := client.StartWorkflowOptions{
				ID:                    WorkflowID,
				TaskQueue:             application.DEFAULT_TASKT_QUEUE,
				WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
			}
			params := application.ReplicateWorkflowParams{
				UUID: id,
			}
			aip, err := app.GetAIPByID(ctx, id.String())
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("get AIP by ID: %v", err)
			} else if aip != nil && aip.Status == string(application.AIPStatusReplicated) {
				slog.Info("AIP Already Replicated")
				continue
			} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
				slog.Info("AIP Not Found")
				continue
			}

			we, err := app.Tc.ExecuteWorkflow(ctx, options, application.ReplicateWorkflowName, params)
			if err != nil {
				slog.Error("workflow launch failed", "err", err)
				continue
			}
			var result application.ReplicateWorkflowResult
			err = we.Get(ctx, &result)
			if err != nil {
				slog.Error("workflow execution failed", "error", err)
				continue
			}
			slog.Info("workflow", "ID", we.GetID())
		}
	case "move":
		for _, id := range UUIDs {
			WorkflowID := fmt.Sprintf("AIP_Move_%s", id.String())
			options := client.StartWorkflowOptions{
				ID:                    WorkflowID,
				TaskQueue:             application.DEFAULT_TASKT_QUEUE,
				WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
			}
			params := application.MoveWorkflowParams{
				UUID: id,
			}
			aip, err := app.GetAIPByID(ctx, id.String())
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("get AIP by ID: %v", err)
			} else if aip != nil && aip.Status == string(application.AIPStatusMoved) {
				slog.Info("AIP Already Moved")
				continue
			} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
				slog.Info("AIP Not Found")
				continue
			}

			we, err := app.Tc.ExecuteWorkflow(ctx, options, application.MoveWorkflowName, params)
			if err != nil {
				slog.Error("workflow launch failed", "err", err)
				continue
			}
			var result application.MoveWorkflowResult
			err = we.Get(ctx, &result)
			if err != nil {
				slog.Error("workflow execution failed", "error", err)
				continue
			}
			slog.Info("workflow", "ID", we.GetID())
		}
	case "index":
	case "export":
		err = app.ExportReplication(ctx)
		if err != nil {
			return fmt.Errorf("export replication: %v", err)
		}
	case "load-input":
		for _, id := range UUIDs {
			_, err := app.InitAIPInDatabase(ctx, id)
			if err != nil {
				return fmt.Errorf("init AIP in database: %v", err)
			}
			_, err = app.FindA(ctx, application.FindParams{AipID: id.String()})
			if err != nil {
				return fmt.Errorf("find AIP: %v", err)
			}
		}

		err = app.ExportReplication(ctx)
		if err != nil {
			return fmt.Errorf("export replication: %v", err)
		}
	}

	return nil
}

func initDatabase(ctx context.Context, datasource string) (db bob.DB, err error) {
	db, err = bob.Open("sqlite", datasource)
	if err != nil {
		return db, fmt.Errorf("open sqlite db: %v", err)
	}

	if err = db.PingContext(ctx); err != nil {
		return db, fmt.Errorf("ping db: %v", err)
	}

	var file []byte
	file, err = migrations.FS.ReadFile("schema.sql")
	if err != nil {
		return db, fmt.Errorf("read schema.sql: %v", err)
	}

	if _, err = db.ExecContext(ctx, string(file)); err != nil {
		return db, fmt.Errorf("exec schema.sql: %v", err)
	}

	return db, nil
}

// RunWorker blocks.
func RunWorker(app *application.App) error {
	w := registerWorker(app)
	return w.Run(worker.InterruptCh())
}

// StartWorker doesn't block.
func StartWorker(app *application.App) error {
	w := registerWorker(app)
	return w.Start()
}

func registerWorker(app *application.App) worker.Worker {
	w := worker.New(app.Tc, application.DEFAULT_TASKT_QUEUE, worker.Options{})
	w.RegisterWorkflowWithOptions(
		application.NewReplicateWorkflow(app).Run,
		workflow.RegisterOptions{
			Name: application.ReplicateWorkflowName,
		})

	w.RegisterWorkflowWithOptions(
		application.NewMoveWorkflow(app).Run,
		workflow.RegisterOptions{
			Name: application.MoveWorkflowName,
		})

	w.RegisterActivity(application.CheckSSConnectionA)
	w.RegisterActivityWithOptions(app.InitAIPInDatabase, activity.RegisterOptions{Name: application.InitAIPInDatabaseName})
	w.RegisterActivityWithOptions(app.ReplicateA, activity.RegisterOptions{Name: application.ReplicateAName})
	w.RegisterActivityWithOptions(app.FindA, activity.RegisterOptions{Name: application.FindAName})
	w.RegisterActivityWithOptions(app.CheckReplicationStatus, activity.RegisterOptions{Name: application.CheckReplicationStatusName})
	w.RegisterActivityWithOptions(app.MoveA, activity.RegisterOptions{Name: application.MoveActivityName})

	return w
}

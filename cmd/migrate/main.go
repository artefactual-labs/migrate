package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/stephenafamo/bob"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/types/known/durationpb"
	_ "modernc.org/sqlite"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/migrations"
	"github.com/artefactual-labs/migrate/internal/storage_service"
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

	cfg, err := application.LoadConfig()
	if err != nil {
		return err
	}

	db, err := initDatabase(ctx, "migrate.db")
	if err != nil {
		return err
	}

	storageClient := storage_service.NewAPI(http.DefaultClient, cfg.SSURL, cfg.SSUserName, cfg.SSAPIKey)

	// Connect with Temporal Server.
	// TODO(daniel): make all these options configurable.
	// TODO: push namespace registration to deployment.
	const temporalNamespace = "move"
	temporalClient, err := client.Dial(client.Options{
		Namespace: temporalNamespace,
		Logger:    logger,
	})
	if err != nil {
		return fmt.Errorf("dial temporal: %v", err)
	}

	nsClient, err := client.NewNamespaceClient(client.Options{Namespace: temporalNamespace})
	if err != nil {
		return fmt.Errorf("new namespace client: %v", err)
	} else if err := nsClient.Register(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace: temporalNamespace,
		WorkflowExecutionRetentionPeriod: &durationpb.Duration{
			Seconds: 31_536_000, /* 365 days. */
		},
	}); err != nil {
		var namespaceAlreadyExists *serviceerror.NamespaceAlreadyExists
		if !errors.As(err, &namespaceAlreadyExists) {
			return fmt.Errorf("register namespace: %v", err)
		}
	}

	app := application.New(logger, db, cfg, temporalClient, storageClient)

	if err := StartWorker(app); err != nil {
		return fmt.Errorf("start worker: %v", err)
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

	if len(args) == 0 {
		return errors.New("missing command")
	}
	command := args[0]

	switch command {
	case "worker":
		err := RunWorker(app)
		if err != nil {
			return fmt.Errorf("run worker: %v", err)
		}
	case "replicate":
		logger.Info("Starting Replication")
		for _, l := range app.Config.ReplicationLocations {
			logger.Info(fmt.Sprintf("Location Name %s, UUID: %s", l.Name, l.UUID))
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
				logger.Info("AIP Already Moved")
				continue
			} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
				logger.Info("AIP Not Found")
				continue
			}

			we, err := app.Tc.ExecuteWorkflow(ctx, options, application.MoveWorkflowName, params)
			if err != nil {
				logger.Error("workflow launch failed", "err", err)
				continue
			}
			var result application.MoveWorkflowResult
			err = we.Get(ctx, &result)
			if err != nil {
				logger.Error("workflow execution failed", "error", err)
				continue
			}
			logger.Info("workflow", "ID", we.GetID())
		}
	case "export":
		if len(args) < 2 {
			return errors.New("missing export type (move|replicate)")
		}
		exportType := strings.ToLower(args[1])
		switch exportType {
		case "move":
			err = app.Export(ctx)
			if err != nil {
				return fmt.Errorf("export move report: %v", err)
			}
		case "replicate":
			err = app.ExportReplication(ctx)
			if err != nil {
				return fmt.Errorf("export replication report: %v", err)
			}
		default:
			return fmt.Errorf("unsupported export type: %s", exportType)
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

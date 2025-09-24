package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/artefactual-labs/migrate/efs"
	"github.com/artefactual-labs/migrate/pkg/application"
)

var pause bool

func main() {
	slog.SetLogLoggerLevel(slog.LevelInfo)

	ctx := context.Background()
	db := initDatabase(ctx, "migrate.db")
	app := &application.App{}
	app.DB = db

	cfgFile, err := os.ReadFile("config.json")
	exitIfErr(err)
	err = json.Unmarshal(cfgFile, &app.Config)
	exitIfErr(err)

	// Connect with Temporal Server.
	// TODO(daniel) make all these options configurable
	tc, err := client.Dial(client.Options{Namespace: "move", Logger: slog.Default()})
	exitIfErr(err)
	nameSpaceClient, err := client.NewNamespaceClient(client.Options{Namespace: "move"})
	exitIfErr(err)
	err = nameSpaceClient.Register(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace:                        "move",
		WorkflowExecutionRetentionPeriod: &durationpb.Duration{Seconds: 31_536_000 /* 365 Days. */},
	})
	exitIfErr(err)
	app.Tc = tc
	err = StartWorker(app)
	exitIfErr(err)

	var input []string
	f, err := os.Open("input.txt")
	application.PanicIfErr(err)
	s := bufio.NewScanner(f)
	for s.Scan() {
		input = append(input, s.Text())
	}
	UUIDs, err := application.ValidateUUIDs(input)
	if err != nil {
		exitIfErr(err)
	}

	var command string
	args := os.Args
	if len(args) <= 1 {
		exitIfErr(errors.New("missing command"))
	}
	command = args[1]
	switch command {
	case "pause":
		pause = true
	case "worker":
		err := RunWorker(app)
		exitIfErr(err)
	case "replicate":
		slog.Info("Starting Replication")
		for _, l := range app.Config.ReplicationLocations {
			slog.Info(fmt.Sprintf("Location Name %s, UUID: %s", l.Name, l.UUID))
		}

		for _, id := range UUIDs {
			if pause {
				break
			}

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
				exitIfErr(err)
			} else if aip != nil && aip.Status == string(application.AIPStatusReplicated) {
				slog.Info("AIP Already Replicated")
				continue
			} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
				slog.Info("AIP Not Found")
				continue
			}

			we, err := tc.ExecuteWorkflow(ctx, options, application.ReplicateWorkflowName, params)
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
			if pause {
				break
			}

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
				exitIfErr(err)
			} else if aip != nil && aip.Status == string(application.AIPStatusMoved) {
				slog.Info("AIP Already Moved")
				continue
			} else if aip != nil && aip.Status == string(application.AIPStatusNotFound) {
				slog.Info("AIP Not Found")
				continue
			}

			we, err := tc.ExecuteWorkflow(ctx, options, application.MoveWorkflowName, params)
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
		exitIfErr(err)
	case "load-input":
		for _, id := range UUIDs {
			_, err := app.InitAIPInDatabase(ctx, id)
			application.PanicIfErr(err)

			_, err = app.FindA(ctx, application.FindParams{AipID: id.String()})
			application.PanicIfErr(err)
		}

		err = app.ExportReplication(ctx)
		application.PanicIfErr(err)
	}
}

func initDatabase(ctx context.Context, datasource string) bob.DB {
	// Immediately connect to database
	dbHandle, err := bob.Open("sqlite", datasource)
	exitIfErr(err)
	err = dbHandle.PingContext(ctx)
	exitIfErr(err)
	file, err := efs.EFS.ReadFile("migrations/schema.sql")
	exitIfErr(err)
	_, err = dbHandle.ExecContext(ctx, string(file))
	exitIfErr(err)
	return dbHandle
}

func exitIfErr(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
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

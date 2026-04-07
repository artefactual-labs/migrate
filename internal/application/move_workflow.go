package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type MoveWorkflowParams struct {
	UUID uuid.UUID
}

type MoveWorkflowResult struct {
	Message     string
	MoveDetails []string
	AIPSize     string
}

const MoveWorkflowName = "move-workflow"

type MoveWorkflow struct {
	App *App
}

func NewMoveWorkflow(app *App) *MoveWorkflow {
	return &MoveWorkflow{App: app}
}

func (w *MoveWorkflow) Run(ctx workflow.Context, params MoveWorkflowParams) (*MoveWorkflowResult, error) {
	result := &MoveWorkflowResult{}

	activityDefaultOptions := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 24 * 365 * 10,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityDefaultOptions)

	var InitResult InitAIPInDatabaseResult
	err := workflow.ExecuteActivity(ctx, InitAIPInDatabaseName, params.UUID).Get(ctx, &InitResult)
	if err != nil {
		return nil, err
	}
	err = workflow.ExecuteActivity(ctx, CheckStorageServiceConnectionActivityName, w.App.Locations).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	findRes := FindResult{}
	err = workflow.ExecuteActivity(ctx, FindAName, FindParams{AipID: params.UUID.String()}).Get(ctx, &findRes)
	if err != nil {
		return nil, err
	}
	result.AIPSize = findRes.Size
	if findRes.Status == string(AIPStatusDeleted) {
		result.Message = "The AIP has been deleted deleted"
		return result, nil
	}

	if w.App.Config.Workflows.Move.CheckFixity {
		fixityParams := FixityActivityParams{UUID: params.UUID.String()}
		fixityResult := FixityActivityResult{}
		err = workflow.ExecuteActivity(ctx, FixityActivityName, fixityParams).Get(ctx, &fixityResult)
		if err != nil {
			return nil, err
		}
		result.MoveDetails = append(result.MoveDetails, "Fixity status: "+fixityResult.Status)
	}

	if w.App.Config.Workflows.Move.OnlyIndexUpdate {
		updateIndexParams := UpdateIndexActivityParams{UUID: params.UUID.String()}
		updateIndexResult := UpdateIndexActivityResult{}
		err = workflow.ExecuteActivity(ctx, UpdateIndexName, updateIndexParams).Get(ctx, &updateIndexResult)
		if err != nil {
			return nil, err
		}
		result.Message = "Only updated Index"
		return result, nil
	}

	if findRes.Status == string(AIPStatusMoved) {
		result.Message = "AIP already moved"
		return result, nil
	}

	moveParams := MoveActivityParams{UUID: params.UUID.String()}
	moveResult := MoveActivityResult{}
	err = workflow.ExecuteActivity(ctx, MoveActivityName, moveParams).Get(ctx, &moveResult)
	if err != nil {
		return nil, err
	}

	updateIndexParams := UpdateIndexActivityParams{UUID: params.UUID.String()}
	updateIndexResult := UpdateIndexActivityResult{}
	err = workflow.ExecuteActivity(ctx, UpdateIndexName, updateIndexParams).Get(ctx, &updateIndexResult)
	if err != nil {
		return nil, err
	}

	result.Message = "Status: " + moveResult.Status
	return result, nil
}

func RunWorkflowFromUUIDs(ctx context.Context, app *App, uuids []uuid.UUID) error {
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
		params := MoveWorkflowParams{
			UUID: id,
		}
		aip, err := app.GetAIPByID(ctx, id.String())
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("get AIP by ID: %w", err)
		} else if aip != nil && aip.Status == string(AIPStatusMoved) {
			slog.Info("AIP Already Moved")
		} else if aip != nil && aip.Status == string(AIPStatusNotFound) {
			slog.Info("AIP Not Found")
			continue
		}

		var we client.WorkflowRun
		for {
			we, err = app.Tc.ExecuteWorkflow(ctx, options, MoveWorkflowName, params)
			if err != nil {
				var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
				if errors.As(err, &alreadyStarted) {
					slog.Info("Workflow already running, retrying shortly.", "workflow_id", workflowID)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(10 * time.Second):
					}
					continue
				}
				slog.Error("Workflow launch failed.", "err", err)
				break
			}
			break
		}
		if we == nil {
			continue
		}
		var result MoveWorkflowResult
		err = we.Get(ctx, &result)
		if err != nil {
			slog.Error("Workflow execution failed.", "error", err)
			continue
		}
		slog.Info("workflow", "ID", we.GetID())
	}
	return nil
}

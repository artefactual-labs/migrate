package application

import (
	"time"

	"github.com/google/uuid"
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

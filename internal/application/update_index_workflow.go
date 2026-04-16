package application

import (
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type UpdateIndexWorkflowParams struct {
	UUID   uuid.UUID
	DryRun bool
}

type UpdateIndexWorkflowResult struct {
	Message string
}

const UpdateIndexWorkflowName = "update-index-workflow"

type UpdateIndexWorkflow struct {
	App *App
}

func NewUpdateIndexWorkflow(app *App) *UpdateIndexWorkflow {
	return &UpdateIndexWorkflow{App: app}
}

func (w *UpdateIndexWorkflow) Run(ctx workflow.Context, params UpdateIndexWorkflowParams) (*UpdateIndexWorkflowResult, error) {
	result := &UpdateIndexWorkflowResult{}

	activityDefaultOptions := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour * 24 * 365 * 10,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityDefaultOptions)

	var initResult InitAIPInDatabaseResult
	err := workflow.ExecuteActivity(ctx, InitAIPInDatabaseName, params.UUID).Get(ctx, &initResult)
	if err != nil {
		return nil, err
	}

	err = workflow.ExecuteActivity(ctx, CheckStorageServiceConnectionActivityName, w.App.Locations).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	updateIndexParams := UpdateIndexActivityParams{
		UUID:   params.UUID.String(),
		DryRun: params.DryRun,
	}
	updateIndexResult := UpdateIndexActivityResult{}
	err = workflow.ExecuteActivity(ctx, UpdateIndexName, updateIndexParams).Get(ctx, &updateIndexResult)
	if err != nil {
		return nil, err
	}

	result.Message = updateIndexResult.Message
	return result, nil
}

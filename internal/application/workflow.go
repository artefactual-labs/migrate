package application

import (
	"context"

	"go.temporal.io/api/workflowservice/v1"
)

func (a *App) IsWorkflowRunning() (bool, error) {
	resp, err := a.Tc.ListOpenWorkflow(context.Background(), &workflowservice.ListOpenWorkflowExecutionsRequest{
		MaximumPageSize: 1,
	})
	if err != nil {
		return false, err
	}
	return len(resp.Executions) > 0, nil
}

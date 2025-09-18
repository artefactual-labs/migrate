package application

import (
	"context"
	"log/slog"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IndexWorkflowParams struct {
	UUID uuid.UUID
}
type IndexWorkflowResult struct {
	Message string
}

const IndexWorkflowName = "index-workflow"

type IndexWorkflow struct {
	App *App
}

func NewIndexWorkflow(app *App) *IndexWorkflow {
	return &IndexWorkflow{App: app}
}

func (w *IndexWorkflow) Run(ctx workflow.Context, params IndexWorkflowParams) (*IndexWorkflowResult, error) {
	result := &IndexWorkflowResult{}

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

	if InitResult.Status == string(AIPStatusIndexed) {
		result.Message = "AIP already Indexed"
		return result, nil
	}

	// err = workflow.ExecuteActivity(ctx, "", params.UUID)

	return result, nil
}

const IndexActivityName = "Index"

type IndexActivityResult struct{}

func (a *App) Index(ctx context.Context, params IndexWorkflowParams) (*IndexActivityResult, error) {
	result := &IndexActivityResult{}

	aip, err := a.GetAIPByID(params.UUID.String())
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	e := StartEvent(ActionIndex)
	var cmd *exec.Cmd = exec.Command(
		a.Config.Dashboard.PythonPath,
		a.Config.Dashboard.ManagePath,
		"rebuild_aip_index_from_storage_service",
		"--delete",
		"--uuid", params.UUID.String(),
	)
	d := a.Config.Dashboard
	cmd.Env = cmd.Environ()
	cmd.Env = append(cmd.Env,
		"PYTHONPATH="+d.PythonPath,
		"LANG="+d.Lang,
		"LC_ALL="+d.Lang,
		"LC_LANG="+d.Lang,
		"DJANGO_SETTINGS_MODULE="+d.DjangoSettingsModule,
		"ARCHIVEMATICA_DASHBOARD_DASHBOARD_DJANGO_ALLOWED_HOSTS="+d.DjangoAllowedHosts,
		"ARCHIVEMATICA_DASHBOARD_DASHBOARD_DJANGO_SECRET_KEY="+d.DjangoSecretKey,
		"AM_GUNICORN_BIND="+d.GunicornBind,
		"ARCHIVEMATICA_DASHBOARD_DASHBOARD_ELASTICSEARCH_SERVER="+d.ElasticSearchServer,
		"ARCHIVEMATICA_DASHBOARD_DASHBOARD_STORAGE_SERVICE_CLIENT_QUICK_TIMEOUT="+d.SSClientQuickTimeout,
	)

	// TODO(daniel): Finish implementation
	EndEvent(AIPStatusIndexed, a, e, aip)
	return result, nil
}

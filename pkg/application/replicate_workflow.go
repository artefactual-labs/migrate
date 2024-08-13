package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"gitlab.artefactual.com/dcosme/migrate/pkg/database/gen/models"
	"gitlab.artefactual.com/dcosme/migrate/pkg/storage_service"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

const DEFAULT_TASKT_QUEUE = "default-task-queue"

type ReplicateWorkflowParams struct {
	UUID uuid.UUID
}
type ReplicateWorkflowResult struct {
	Message          string
	ReplicateDetails []string
	AIPSize          string
}

const ReplicateWorkflowName = "replicate-workflow"

type ReplicateWorkflow struct {
	App *App
}

func NewReplicateWorkflow(app *App) *ReplicateWorkflow {
	return &ReplicateWorkflow{App: app}
}

func (w *ReplicateWorkflow) Run(ctx workflow.Context, params ReplicateWorkflowParams) (*ReplicateWorkflowResult, error) {
	result := &ReplicateWorkflowResult{}

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

	if InitResult.Status == string(AIPStatusReplicated) {
		result.Message = "AIP already replicated"
		return result, nil
	}

	err = workflow.ExecuteActivity(ctx, CheckSSConnectionA, w.App.Config).Get(ctx, nil)
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

	for _, repl := range InitResult.DesiredReplication {
		var replicateResult ReplicateResult
		replicateParams := ReplicateParams{
			AipID:               params.UUID.String(),
			LocationUUID:        w.App.Config.LocationUUID,
			ReplicaLocationUUID: repl,
		}
		err = workflow.ExecuteActivity(ctx, ReplicateAName, replicateParams).Get(ctx, &replicateResult)
		if err != nil {
			return nil, err
		}
		result.ReplicateDetails = append(result.ReplicateDetails, replicateResult.Details...)
	}

	// TODO(daniel): Implement AIP Status reconciliation based on the workflow.
	err = workflow.ExecuteActivity(ctx, CheckReplicationStatusName, CheckReplicationStatusParams{AIP_UUID: params.UUID.String()}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

const InitAIPInDatabaseName = "init_AIP_in_database"

type InitAIPInDatabaseResult struct {
	Status             string
	DesiredReplication []string
}

func (a *App) InitAIPInDatabase(ctx context.Context, id uuid.UUID) (*InitAIPInDatabaseResult, error) {
	result := &InitAIPInDatabaseResult{}
	aipSetter := &models.AipSetter{
		UUID:   omit.From(id.String()),
		Status: omit.From(string(AIPStatusNew)),
	}
	aip, err := models.Aips.Upsert(ctx, a.DB, false, []string{"uuid"}, nil, aipSetter)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if aip, err = models.Aips.Query(ctx, a.DB, models.SelectWhere.Aips.UUID.EQ(id.String())).One(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	result.Status = aip.Status
	if err := aip.LoadAipAipReplications(ctx, a.DB); err != nil {
		return nil, err
	}
	if aip.Status == string(AIPStatusNew) && len(aip.R.AipReplications) == 0 {
		for _, l := range a.Config.ReplicationLocations {
			replicationLocationSetter := models.AipReplicationSetter{
				AipID:        omit.From(aip.ID),
				LocationUUID: omitnull.From(l.UUID),
				Status:       omit.From(string(AIPReplicationStatusNew)),
			}
			if err := aip.InsertAipReplications(ctx, a.DB, &replicationLocationSetter); err != nil {
				return nil, err
			}
		}
	}
	if err := aip.LoadAipAipReplications(ctx, a.DB); err != nil {
		return nil, err
	}
	for _, rl := range aip.R.AipReplications {
		result.DesiredReplication = append(result.DesiredReplication, rl.LocationUUID.GetOrZero())
	}
	result.Status = aip.Status
	return result, nil
}

type ReplicateParams struct {
	AipID               string
	LocationUUID        string
	ReplicaLocationUUID string
}
type ReplicateResult struct {
	Command string
	Details []string
	Status  string
}

const ReplicateAName = "Replicate-aip"

func (a *App) ReplicateA(ctx context.Context, params ReplicateParams) (*ReplicateResult, error) {
	result := &ReplicateResult{}

	aip, err := a.GetAIPByID(params.AipID)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	e := StartEvent(ActionReplicate)
	ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
	ssPackage, err := ssAPI.Packages.GetByID(aip.UUID)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(ssPackage.Status) == "deleted" {
		slog.Info("AIP has been deleted")
		result.Status = string(AIPStatusDeleted)
		EndEvent(AIPStatusDeleted, a, e, aip)
		return result, nil
	}

	d1 := fmt.Sprintf("Number of current replicas: %d", len(ssPackage.Replicas))
	e.AddDetail(d1)
	result.Details = append(result.Details, d1)

	var cmd *exec.Cmd
	if a.Config.Docker {
		cmd = exec.Command(
			"docker",
			"exec",
			a.Config.SSContainerName,
			a.Config.SSManagePath,
			"create_aip_replicas",
			"--aip-uuid", aip.UUID,
			"--aip-store-location", params.LocationUUID,
			"--replicator-location", params.ReplicaLocationUUID,
		)
	} else {
		cmd = exec.Command(
			a.Config.PythonPath,
			a.Config.SSManagePath,
			"create_aip_replicas",
			"--aip-uuid", aip.UUID,
			"--aip-store-location", params.LocationUUID,
			"--replicator-location", params.ReplicaLocationUUID,
		)
		cmd.Env = cmd.Environ()
		cmd.Env = append(cmd.Env,
			"DJANGO_SETTINGS_MODULE="+a.Config.Environment.DjangoSettingsModule,
			"DJANGO_SECRET_KEY="+a.Config.Environment.DjangoSecretKey,
			"DJANGO_ALLOWED_HOSTS="+a.Config.Environment.DjangoAllowedHosts,
			"SS_GUNICORN_BIND="+a.Config.Environment.SsGunicornBind,
			"EMAIL_HOST="+a.Config.Environment.EmailHost,
			"SS_AUDIT_LOG_MIDDLEWARE="+a.Config.Environment.SsAuditLogMiddleware,
			"SS_DB_URL="+a.Config.Environment.SsDbUrl,
			"EMAIL_USE_TLS="+a.Config.Environment.EmailUseTls,
			"SS_PROMETHEUS_ENABLED="+a.Config.Environment.SsPrometheusEnabled,
			"DEFAULT_FROM_EMAIL="+a.Config.Environment.DefaultFromEmail,
			"TIME_ZONE="+a.Config.Environment.TimeZone,
			"SS_GUNICORN_WORKERS="+a.Config.Environment.SsGunicornWorkers,
			"REQUESTS_CA_BUNDLE="+a.Config.Environment.RequestsCaBundle,
		)
	}

	aipReplication, err := models.AipReplications.Query(
		ctx, a.DB,
		models.SelectWhere.AipReplications.AipID.EQ(aip.ID),
		models.SelectWhere.AipReplications.LocationUUID.EQ(params.ReplicaLocationUUID),
	).One()
	if err != nil {
		return nil, err
	}
	if aipReplication.Status == string(AIPReplicationStatusFinished) {
		result.Status = aipReplication.Status
		return result, nil
	}
	a.UpdateAIPStatus(aip.ID, AIPStatusReplicationInProgress)

	// TODO(daniel): Mark AIP Replication as In Progress.
	//		Add attempt ++1
	attempt := aipReplication.Attempt
	attempt++
	if err := aipReplication.Update(ctx, a.DB, &models.AipReplicationSetter{
		Status:  omit.From(string(AIPReplicationStatusInProgress)),
		Attempt: omit.From(attempt),
	}); err != nil {
		return nil, err
	}

	result.Command = cmd.String()
	slog.Info("Replicating AIP", "command", cmd.String())

	if output, err := cmd.CombinedOutput(); err != nil {
		a.updateReplicateAIPStatus(aipReplication, AIPReplicationStatusFailed)
		e.AddDetail(string(output))
		result.Details = append(result.Details, string(output))
		EndEventErr(a, e, aip, err.Error())
		return nil, err
	} else {
		e.AddDetail(string(output))
		res := strings.Split(string(output), "\n")
		result.Details = append(result.Details, res...)
		if len(res) > 0 {
			sentence := res[len(res)-2]
			e.AddDetail("Sentence: " + sentence)
			if strings.Contains(sentence, "New replicas created for 1 of 1 AIPs in location") {
				// TODO(daniel): Mark AIP Replication as Replicated.
				result.Status = string(AIPReplicationStatusFinished)
				a.updateReplicateAIPStatus(aipReplication, AIPReplicationStatusFinished)
				EndEventNoChange(a, e, aip)
			} else if strings.Contains(sentence, "New replicas created for 0 of 1 AIPs in location.") {
				// TODO(daniel): Mark AIP Replication as Stalled/Unknown.
				a.updateReplicateAIPStatus(aipReplication, AIPReplicationStatusUnknown)
				e.AddDetail("Not replicated")
				EndEventErr(a, e, aip, sentence)
				return nil, err
			} else if strings.Contains(sentence, "CommandError: No AIPs to replicate in location") {
				// NOTE: In this case AIP has been deleted.
				a.updateReplicateAIPStatus(aipReplication, AIPReplicationStatusFailed)
				e.AddDetail("Not replicated")
				EndEventErr(a, e, aip, sentence)
				EndEvent(AIPStatusDeleted, a, e, aip)
			}
		} else {
			// TODO(daniel): Mark AIP Replication as Stalled/Unknown.
			a.updateReplicateAIPStatus(aipReplication, AIPReplicationStatusUnknown)
			slog.Info("Replication command returned", "output", string(output))
			EndEventErr(a, e, aip, "Could not determine result of Replication")
			return nil, errors.New("could not determine result of replication")
		}
	}

	return result, nil
}

type FindParams struct {
	AipID string
}
type FindResult struct {
	Size   string
	Status string
}

const FindAName = "find-aip"

func (a *App) FindA(ctx context.Context, params FindParams) (*FindResult, error) {
	result := &FindResult{}
	aip, err := a.GetAIPByID(params.AipID)
	if err != nil {
		return nil, err
	}
	result.Status = aip.Status
	if aip.Status != string(AIPReplicationStatusNew) {
		result.Size = FormatByteSize(aip.Size.GetOrZero())
		return result, nil
	}
	err = find(a, aip)
	if err != nil {
		return nil, err
	}
	aip.Reload(ctx, a.DB)
	result.Size = FormatByteSize(aip.Size.GetOrZero())
	result.Status = aip.Status
	return result, nil
}

const CheckReplicationStatusName = "check-replication-status"

type CheckReplicationStatusParams struct {
	AIP_UUID string
}

func (a *App) CheckReplicationStatus(ctx context.Context, params CheckReplicationStatusParams) error {
	q := models.Aips.Query(
		ctx,
		a.DB,
		models.SelectWhere.Aips.UUID.EQ(params.AIP_UUID),
	)
	q.Apply(models.ThenLoadAipAipReplications())
	aip, err := q.One()
	if err != nil {
		return err
	}

	finishedCount := 0
	for _, r := range aip.R.AipReplications {
		slog.Info("AIP Replication", "Status", r.Status, "AIP UUID", aip.UUID)
		if r.Status == string(AIPReplicationStatusFinished) {
			finishedCount++
		}
	}
	if len(aip.R.AipReplications) == finishedCount {
		a.UpdateAIPStatus(aip.ID, AIPStatusReplicated)
		return nil
	}
	return errors.New("cannot determine final status of replication")
}

func CheckSSConnectionA(ctx context.Context, config Config) error {
	ssAPI := storage_service.NewAPI(config.SSURL, config.SSUserName, config.SSAPIKey)
	for _, l := range config.ReplicationLocations {
		loc, err := ssAPI.Location.Get(l.UUID)
		if err != nil {
			return fmt.Errorf("error connecting with the SS: %w", err)
		}
		slog.Info("Location found: " + loc.Description + "- Purpose: " + loc.Purpose)
	}
	slog.Info("Connection to SS working")
	return nil
}

func (a *App) updateReplicateAIPStatus(aip *models.AipReplication, status AIPReplicationStatus) {
	if err := aip.Update(context.Background(), a.DB, &models.AipReplicationSetter{
		Status: omit.From(string(status)),
	}); err != nil {
		slog.Error(err.Error())
		panic(err)
	}
}

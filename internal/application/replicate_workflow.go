package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/sqlite/im"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

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

	for _, repl := range InitResult.DesiredReplication {
		var replicateResult ReplicateResult
		replicateParams := ReplicateParams{
			AipID:               params.UUID.String(),
			LocationUUID:        w.App.Locations.SourceLocationID,
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
	aip, err := models.Aips.Insert(
		aipSetter,
		im.OnConflict("uuid").DoNothing(),
	).One(ctx, a.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			q := models.Aips.Query(models.SelectWhere.Aips.UUID.EQ(id.String()))
			if aip, err = q.One(ctx, a.DB); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	result.Status = aip.Status
	if err := aip.LoadAipReplications(ctx, a.DB); err != nil {
		return nil, err
	}
	if aip.Status == string(AIPStatusNew) && len(aip.R.AipReplications) == 0 {
		for _, l := range a.Locations.ReplicationTargets {
			replicationLocationSetter := models.AipReplicationSetter{
				AipID:        omit.From(aip.ID),
				LocationUUID: omitnull.From(l.ID),
				Status:       omit.From(string(AIPReplicationStatusNew)),
			}
			if err := aip.InsertAipReplications(ctx, a.DB, &replicationLocationSetter); err != nil {
				return nil, err
			}
		}
	}
	if err := aip.LoadAipReplications(ctx, a.DB); err != nil {
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
	logger := activity.GetLogger(ctx)
	result := &ReplicateResult{}

	aip, err := a.GetAIPByID(ctx, params.AipID)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	e := StartEvent(ActionReplicate)
	ssPackage, err := a.StorageClient.Packages.GetByID(ctx, aip.UUID)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(ssPackage.Status) == "deleted" {
		logger.Info("AIP has been deleted")
		result.Status = string(AIPStatusDeleted)
		if eventErr := EndEvent(ctx, AIPStatusDeleted, a, e, aip); eventErr != nil {
			return nil, eventErr
		}
		return result, nil
	}

	d1 := fmt.Sprintf("Number of current replicas: %d", len(ssPackage.Replicas))
	e.AddDetail(d1)
	result.Details = append(result.Details, d1)

	var cmd *exec.Cmd
	management := a.Config.StorageService.Management
	switch management.Mode {
	case "docker":
		container := management.Docker.Container
		if container == "" {
			return nil, fmt.Errorf("storage_service.management.docker.container is required")
		}
		managePath := management.Docker.ManagePath
		if managePath == "" {
			return nil, fmt.Errorf("storage_service.management.docker.manage_path is required")
		}
		cmd = exec.Command(
			"docker",
			"exec",
			container,
			managePath,
			"create_aip_replicas",
			"--aip-uuid", aip.UUID,
			"--aip-store-location", params.LocationUUID,
			"--replicator-location", params.ReplicaLocationUUID,
		)
	case "host":
		managePath := management.Host.ManagePath
		if managePath == "" {
			return nil, fmt.Errorf("storage_service.management.host.manage_path is required")
		}
		pythonPath := management.Host.PythonPath
		if pythonPath == "" {
			pythonPath = "python3"
		}
		cmd = exec.Command(
			pythonPath,
			managePath,
			"create_aip_replicas",
			"--aip-uuid", aip.UUID,
			"--aip-store-location", params.LocationUUID,
			"--replicator-location", params.ReplicaLocationUUID,
		)
		cmd.Env = cmd.Environ()
		if len(management.Host.Environment) > 0 {
			keys := make([]string, 0, len(management.Host.Environment))
			for key := range management.Host.Environment {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, management.Host.Environment[key]))
			}
		}
	default:
		return nil, fmt.Errorf("unsupported storage service management mode %q", management.Mode)
	}

	q := models.AipReplications.Query(
		models.SelectWhere.AipReplications.AipID.EQ(aip.ID),
		models.SelectWhere.AipReplications.LocationUUID.EQ(params.ReplicaLocationUUID),
	)
	aipReplication, err := q.One(ctx, a.DB)
	if err != nil {
		return nil, err
	}
	if aipReplication.Status == string(AIPReplicationStatusFinished) {
		result.Status = aipReplication.Status
		return result, nil
	}
	if err := a.UpdateAIPStatus(ctx, aip.ID, AIPStatusReplicationInProgress); err != nil {
		return nil, err
	}

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
	logger.Info("Replicating AIP", "command", cmd.String())

	if output, err := cmd.CombinedOutput(); err != nil {
		if updateErr := a.updateReplicateAIPStatus(ctx, aipReplication, AIPReplicationStatusFailed); updateErr != nil {
			return nil, errors.Join(err, updateErr)
		}
		e.AddDetail(string(output))
		result.Details = append(result.Details, string(output))
		if eventErr := EndEventErr(ctx, a, e, aip, err.Error()); eventErr != nil {
			return nil, errors.Join(err, eventErr)
		}
		logger.Error("ERROR", "error", err.Error(), "output", string(output))
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
				if err := a.updateReplicateAIPStatus(ctx, aipReplication, AIPReplicationStatusFinished); err != nil {
					return nil, err
				}
				if err := EndEventNoChange(ctx, a, e, aip); err != nil {
					return nil, err
				}
			} else if strings.Contains(sentence, "New replicas created for 0 of 1 AIPs in location.") {
				// TODO(daniel): Mark AIP Replication as Stalled/Unknown.
				if err := a.updateReplicateAIPStatus(ctx, aipReplication, AIPReplicationStatusUnknown); err != nil {
					return nil, err
				}
				e.AddDetail("Not replicated")
				if eventErr := EndEventErr(ctx, a, e, aip, sentence); eventErr != nil {
					return nil, eventErr
				}
				return nil, err
			} else if strings.Contains(sentence, "CommandError: No AIPs to replicate in location") {
				// NOTE: In this case AIP has been deleted.
				if err := a.updateReplicateAIPStatus(ctx, aipReplication, AIPReplicationStatusFailed); err != nil {
					return nil, err
				}
				e.AddDetail("Not replicated")
				if eventErr := EndEventErr(ctx, a, e, aip, sentence); eventErr != nil {
					return nil, eventErr
				}
				if eventErr := EndEvent(ctx, AIPStatusDeleted, a, e, aip); eventErr != nil {
					return nil, eventErr
				}
			}
		} else {
			// TODO(daniel): Mark AIP Replication as Stalled/Unknown.
			if err := a.updateReplicateAIPStatus(ctx, aipReplication, AIPReplicationStatusUnknown); err != nil {
				return nil, err
			}
			logger.Info("Replication command returned", "output", string(output))
			if eventErr := EndEventErr(ctx, a, e, aip, "Could not determine result of Replication"); eventErr != nil {
				return nil, eventErr
			}
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
	aip, err := a.GetAIPByID(ctx, params.AipID)
	if err != nil {
		return nil, err
	}
	result.Status = aip.Status
	if aip.Status != string(AIPReplicationStatusNew) {
		result.Size = formatByteSize(aip.Size.GetOrZero())
		return result, nil
	}
	err = find(ctx, a.logger, a, a.StorageClient, aip)
	if err != nil {
		return nil, err
	}
	if err := aip.Reload(ctx, a.DB); err != nil {
		return nil, err
	}
	result.Size = formatByteSize(aip.Size.GetOrZero())
	result.Status = aip.Status
	return result, nil
}

const CheckReplicationStatusName = "check-replication-status"

type CheckReplicationStatusParams struct {
	AIP_UUID string
}

func (a *App) CheckReplicationStatus(ctx context.Context, params CheckReplicationStatusParams) error {
	logger := activity.GetLogger(ctx)
	q := models.Aips.Query(
		models.SelectWhere.Aips.UUID.EQ(params.AIP_UUID),
	)
	q.Apply(models.SelectThenLoad.Aip.AipReplications())
	aip, err := q.One(ctx, a.DB)
	if err != nil {
		return err
	}

	finishedCount := 0
	for _, r := range aip.R.AipReplications {
		logger.Info("AIP Replication", "Status", r.Status, "AIP UUID", aip.UUID)
		if r.Status == string(AIPReplicationStatusFinished) {
			finishedCount++
		}
	}
	if len(aip.R.AipReplications) == finishedCount {
		if err := a.UpdateAIPStatus(ctx, aip.ID, AIPStatusReplicated); err != nil {
			return err
		}
		return nil
	}
	return errors.New("cannot determine final status of replication")
}

const CheckStorageServiceConnectionActivityName = "check-storage-service-connection"

type CheckStorageServiceConnectionActivity struct {
	StorageClient *storage_service.API
}

func NewCheckStorageServiceConnectionActivity(storageClient *storage_service.API) *CheckStorageServiceConnectionActivity {
	return &CheckStorageServiceConnectionActivity{StorageClient: storageClient}
}

func (a *CheckStorageServiceConnectionActivity) Execute(ctx context.Context, locations StorageServiceLocationConfig) error {
	logger := activity.GetLogger(ctx)
	for _, l := range locations.ReplicationTargets {
		loc, err := a.StorageClient.Location.Get(ctx, l.ID)
		if err != nil {
			return fmt.Errorf("error connecting with the SS: %w", err)
		}
		logger.Info("Location found: " + loc.Description + "- Purpose: " + loc.Purpose)
	}
	logger.Info("Connection to SS working")
	return nil
}

func (a *App) updateReplicateAIPStatus(ctx context.Context, aip *models.AipReplication, status AIPReplicationStatus) error {
	if err := aip.Update(ctx, a.DB, &models.AipReplicationSetter{
		Status: omit.From(string(status)),
	}); err != nil {
		return err
	}
	return nil
}

// find checks if the AIPs exist in the Storage Service and updates their status
// accordingly.
func find(ctx context.Context, logger *slog.Logger, a *App, storageClient *storage_service.API, aips ...*models.Aip) error {
	logger.Info(fmt.Sprintf("Finding %d AIPS", len(aips)))
	for _, aip := range aips {
		e := StartEvent(ActionFind)
		ssPackage, err := storageClient.Packages.GetByID(ctx, aip.UUID)
		if err != nil {
			if errors.Is(err, storage_service.ErrNotFound) {
				if eventErr := EndEventErr(ctx, a, e, aip, "AIP not found in Storage Service"); eventErr != nil {
					return eventErr
				}
				if err := a.UpdateAIPStatus(ctx, aip.ID, AIPStatusNotFound); err != nil {
					return err
				}
				continue
			}
			return err
		}

		logger.Info("AIP found", "UUID", ssPackage.UUID)
		sizeVal := omitnull.Val[int64]{}
		if ssPackage.Size > math.MaxInt64 {
			logger.Warn("package size exceeds supported range", "uuid", ssPackage.UUID, "size", ssPackage.Size)
		} else {
			sizeVal = omitnull.From(int64(ssPackage.Size))
		}

		found := true
		status := AIPStatusFound
		if ssPackage.Status == "Deleted" {
			found = false
			status = AIPStatusDeleted
		}
		if err := a.UpdateAIP(ctx, aip.ID,
			&models.AipSetter{
				Found: omit.From(found),
				Size:  sizeVal,
			},
		); err != nil {
			return err
		}
		if err := EndEvent(ctx, status, a, e, aip); err != nil {
			return err
		}
	}
	return nil
}

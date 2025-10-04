package application

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"go.temporal.io/sdk/client"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

type App struct {
	logger *slog.Logger
	DB     bob.DB
	Config *Config
	Tc     client.Client

	// A client to interact with the Storage Service API.
	StorageClient *storage_service.API
}

func New(logger *slog.Logger, db bob.DB, cfg *Config, temporalClient client.Client, storageClient *storage_service.API) *App {
	return &App{
		logger:        logger,
		DB:            db,
		Config:        cfg,
		Tc:            temporalClient,
		StorageClient: storageClient,
	}
}

type AIPStatus string

const (
	AIPStatusNew                   AIPStatus = "new"
	AIPStatusFound                 AIPStatus = "found"
	AIPStatusNotFound              AIPStatus = "not-found"
	AIPStatusNoOp                  AIPStatus = "no-op"
	AIPStatusFailed                AIPStatus = "failed"
	AIPStatusFixityChecked         AIPStatus = "fixity-checked"
	AIPStatusMoving                AIPStatus = "moving"
	AIPStatusMoved                 AIPStatus = "moved"
	AIPStatusCleaned               AIPStatus = "cleaned"
	AIPStatusReplicated            AIPStatus = "replicated"
	AIPStatusIndexed               AIPStatus = "indexed"
	AIPStatusReplicationInProgress AIPStatus = "replication-in-progress"
	AIPStatusFinished              AIPStatus = "finished"
	AIPStatusDeleted               AIPStatus = "deleted"
)

type AIPReplicationStatus string

const (
	AIPReplicationStatusNew        AIPReplicationStatus = "new"
	AIPReplicationStatusInProgress AIPReplicationStatus = "in-progress"
	AIPReplicationStatusUnknown    AIPReplicationStatus = "unknown"
	AIPReplicationStatusFailed     AIPReplicationStatus = "failed"
	AIPReplicationStatusFinished   AIPReplicationStatus = "finished"
)

func (a *App) Export(ctx context.Context) error {
	err := os.RemoveAll("move-report.csv")
	if err != nil {
		return err
	}
	file, err := os.Create("move-report.csv")
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{
		"UUID",
		"AIPStatus",
		"Duration",
		"fixity-run",
		"moved",
		"cleaned",
		"replicated",
		"re-indexed",
		"size",
		"Duration Nanoseconds",
		"New Path",
		"Old Path",
		"Replica UUID",
		"Local copy Path",
		"Staged Copy Path",
		"Errors",
	}

	q := models.Aips.Query()
	q.Apply(models.SelectThenLoad.Aip.Errors())
	aips, err := q.All(ctx, a.DB)
	if err != nil {
		return err
	}

	data := make([][]string, len(aips))
	for idx, aip := range aips {
		errs := []string{}
		for _, e := range aip.R.Errors {
			errs = append(errs, e.MSG)
		}

		row := []string{
			aip.UUID,
			aip.Status,
			// aip.TotalDuration.GetOrZero(),
			formatBool(aip.FixityRun),
			formatBool(aip.Moved),
			formatBool(aip.Cleaned),
			formatBool(aip.Replicated),
			formatBool(aip.ReIndexed),
			formatByteSize(aip.Size.GetOrZero()),
			// fmt.Sprintf("%d", aip.TotalDurationNanosecond.GetOrZero()),
			// aip.NewFullPath.GetOrZero(),
			// aip.OldFullPath.GetOrZero(),
			// aip.ReplicaPath.GetOrZero(),
			// aip.LocalCopyPath.GetOrZero(),
			// aip.StagedCopyPath.GetOrZero(),
			strings.Join(errs, "-\n"),
		}
		data[idx] = row
	}

	err = writer.Write(headers)
	if err != nil {
		return err
	}

	for _, row := range data {
		err = writer.Write(row)
		if err != nil {
			return err
		}
	}
	a.logger.Info("Move export generated", "path", "move-report.csv")
	return nil
}

func (a *App) ExportReplication(ctx context.Context) error {
	err := os.RemoveAll("replication-report.csv")
	if err != nil {
		return err
	}

	file, err := os.Create("replication-report.csv")
	if err != nil {
		return err
	}

	defer file.Close() //nolint:errcheck

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{
		"UUID",
		"AIPStatus",
		"Location",
		"Size",
		"Size Bytes",
		"Total Size",
	}

	q := models.Aips.Query()
	q.Apply(models.SelectThenLoad.Aip.Errors())
	q.Apply(models.SelectThenLoad.Aip.Events())
	aips, err := q.All(ctx, a.DB)
	if err != nil {
		return err
	}
	SortAips(aips)

	data := make([][]string, len(aips)+1)

	var totalSize int64
	for idx, aip := range aips {
		// TODO: aip.R.Events and aip.R.Errors?
		totalSize += aip.Size.GetOrZero()
		row := []string{
			aip.UUID,
			aip.Status,
			aip.CurrentLocation.GetOrZero(),
			formatByteSize(aip.Size.GetOrZero()),
			fmt.Sprintf("%d", aip.Size.GetOrZero()),
		}

		data[idx] = row
	}
	data[len(aips)] = []string{"", "", "", "", formatByteSize(totalSize)}

	err = writer.Write(headers)
	if err != nil {
		return err
	}
	for _, row := range data {
		err = writer.Write(row)
		if err != nil {
			return err
		}
	}
	a.logger.Info("Success!")
	return nil
}

func (a *App) GetAIPByID(ctx context.Context, uuid string) (*models.Aip, error) {
	q := models.Aips.Query(models.SelectWhere.Aips.UUID.EQ(uuid))
	return q.One(ctx, a.DB)
}

func (a *App) UpdateAIP(ctx context.Context, id int64, setter *models.AipSetter) error {
	_, err := models.Aips.Update(
		setter.UpdateMod(),
		models.UpdateWhere.Aips.ID.EQ(id),
	).Exec(ctx, a.DB)

	return err
}

func (a *App) UpdateAIPStatus(ctx context.Context, id int64, s AIPStatus) error {
	setter := &models.AipSetter{Status: omit.From(string(s))}
	switch s {
	case AIPStatusMoved:
		setter.Moved = omit.From(true)
	case AIPStatusCleaned:
		setter.Cleaned = omit.From(true)
	case AIPStatusReplicated:
		setter.Replicated = omit.From(true)
	case AIPStatusIndexed:
		setter.ReIndexed = omit.From(true)
	case AIPStatusFixityChecked:
		setter.FixityRun = omit.From(true)
	}
	_, err := models.Aips.Update(
		setter.UpdateMod(),
		models.UpdateWhere.Aips.ID.EQ(id),
	).Exec(ctx, a.DB)
	if err != nil {
		return err
	}
	a.logger.Info("AIP Updated", "AIPStatus", s)
	return nil
}

func (a *App) AddAIPError(ctx context.Context, aip *models.Aip, msg string, details ...string) {
	a.logger.Error(msg, "AIP ID", aip.UUID)
	err := aip.InsertErrors(
		ctx,
		a.DB,
		&models.ErrorSetter{MSG: omit.FromCond(msg, msg != ""), Details: omitnull.From(strings.Join(details, "-"))},
	)
	if err != nil {
		a.logger.Error("failed persisting error", "err", err.Error(), "aip_UUID", aip.UUID)
	}
}

func ValidateUUIDs(input []string) (uuids []uuid.UUID, err error) {
	for _, id := range input {
		res, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		uuids = append(uuids, res)
	}
	return uuids, nil
}

func formatBool(b bool) string {
	if b {
		return "Done"
	}
	return "Not Done"
}

func formatByteSize(b int64) string {
	const unit = 1024

	// Guard against negative inputs.
	if b < 0 {
		return "0 B"
	}

	// Bytes case.
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

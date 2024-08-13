package application

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"gitlab.artefactual.com/dcosme/migrate/pkg/database/gen/models"
	"gitlab.artefactual.com/dcosme/migrate/pkg/storage_service"
	"go.temporal.io/sdk/client"
	"log/slog"
	"os"
	"strings"
)

type App struct {
	DB     bob.DB
	Config Config
	Tc     client.Client
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

const ()

/*
func (a *App) RunDaemonBatch(input []string) error {
	if err := CheckSSConnection(a); err != nil {
		return err
	}
	if err := ProcessUUIDInput(a, input); err != nil {
		return err
	}

	// Find AIPs.
	aips, err := a.GetAIPsByStatus(StatusNew)
	if err != nil {
		return err
	}
	if err := find(a, aips...); err != nil {
		return err
	}

	if a.Config.CheckFixity {
		aips, err = a.GetAIPsByStatus(StatusFound)
		if err != nil {
			return err
		}
		if err := checkFixity(a, aips...); err != nil {
			return err
		}
	}

	// Move AIPs
	if a.Config.Move {
		aips, err = a.GetAIPsByStatus(StatusFixityChecked, StatusMoving)
		if err != nil {
			return err
		}
		if err := move(a, aips...); err != nil {
			return err
		}
	}

	// Replicate AIPs.
	if a.Config.Replicate {
		aips, err = a.GetAIPsByStatus(StatusMoved)
		if err != nil {
			return err
		}
		if err := Replicate(a, true, aips...); err != nil {
			return err
		}
	}

	// Clean AIPs.
	if a.Config.Clean {
		aips, err = a.GetAIPsByStatus(AIPStatusReplicated)
		if err != nil {
			return err
		}
		if err := clean(a, aips...); err != nil {
			return err
		}
	}

	// Re-Index AIPs.
	if a.Config.ReIndex {
		aips, err := a.GetAIPsByStatus(AIPStatusReplicated, StatusMoved, StatusCleaned)
		if err != nil {
			return err
		}
		if err := reindex(a, aips...); err != nil {
			return err
		}
	}

	return nil
}
*/

func (a *App) Export() error {
	err := os.RemoveAll("report.csv")
	PanicIfErr(err)
	file, err := os.Create("report.csv")
	PanicIfErr(err)
	defer file.Close()

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

	q := models.Aips.Query(context.Background(), a.DB)
	q.Apply(models.ThenLoadAipErrors())
	aips, err := q.All()

	PanicIfErr(err)
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
			FormatByteSize(aip.Size.GetOrZero()),
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
	PanicIfErr(err)
	for _, row := range data {
		err = writer.Write(row)
		PanicIfErr(err)
	}
	slog.Info("Success!")
	return nil
}

func (a *App) ExportReplication() error {
	err := os.RemoveAll("replication-report.csv")
	PanicIfErr(err)
	file, err := os.Create("replication-report.csv")
	PanicIfErr(err)
	defer file.Close()

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

	q := models.Aips.Query(context.Background(), a.DB)
	q.Apply(models.ThenLoadAipErrors())
	q.Apply(models.ThenLoadAipEvents())
	aips, err := q.All()
	PanicIfErr(err)
	SortAips(aips)

	data := make([][]string, len(aips)+1)

	var totalSize uint64
	for idx, aip := range aips {
		errs := []string{}
		events := []string{}
		for _, e := range aip.R.Errors {
			errs = append(errs, e.MSG)
		}
		for _, e := range aip.R.Events {
			de := []string{}
			err := json.Unmarshal([]byte(e.Details.GetOrZero()), &de)
			PanicIfErr(err)
			events = append(events, de...)
		}

		totalSize += aip.Size.GetOrZero()
		row := []string{
			aip.UUID,
			aip.Status,
			aip.CurrentLocation.GetOrZero(),
			FormatByteSize(aip.Size.GetOrZero()),
			fmt.Sprintf("%d", aip.Size.GetOrZero()),
		}

		data[idx] = row
	}
	data[len(aips)] = []string{"", "", "", "", FormatByteSize(totalSize)}

	err = writer.Write(headers)
	PanicIfErr(err)
	for _, row := range data {
		err = writer.Write(row)
		PanicIfErr(err)
	}
	slog.Info("Success!")
	return nil
}

func (a *App) GetAIPs() (models.AipSlice, error) {
	return models.Aips.Query(context.Background(), a.DB).All()
}

func (a *App) GetAIPByID(uuid string) (*models.Aip, error) {
	q := models.Aips.Query(context.Background(), a.DB)
	q.Apply(models.SelectWhere.Aips.UUID.EQ(uuid))
	return q.One()
}

func (a *App) GetAIPsByStatus(ss ...AIPStatus) (models.AipSlice, error) {
	q := models.Aips.Query(context.Background(), a.DB)
	var args []string
	for _, s := range ss {
		args = append(args, string(s))
	}
	q.Apply(models.SelectWhere.Aips.Status.In(args...))
	return q.All()
}

func (a *App) UpdateAIP(id int32, setter *models.AipSetter) {
	err := models.Aips.Update(context.Background(), a.DB, setter, &models.Aip{ID: id})
	PanicIfErr(err)
}

func (a *App) UpdateAIPStatus(id int32, s AIPStatus) {
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
	err := models.Aips.Update(
		context.Background(),
		a.DB,
		setter,
		&models.Aip{ID: id},
	)
	PanicIfErr(err)
	slog.Info("AIP Updated", "AIPStatus", s)
}

func (a *App) AddAIPError(aip *models.Aip, msg string, details ...string) {
	slog.Error(msg, "AIP ID", aip.UUID)
	err := aip.InsertErrors(
		context.Background(),
		a.DB,
		&models.ErrorSetter{MSG: omit.FromCond(msg, msg != ""), Details: omitnull.From(strings.Join(details, "-"))},
	)
	if err != nil {
		slog.Error("failed persisting error", "err", err.Error(), "aip_UUID", aip.UUID)
	}
}

func ValidateUUIDs(input []string) (uuids []uuid.UUID, err error) {
	slog.Info("validating uuids", "amount", len(input))
	for idx, id := range input {
		res, err := uuid.Parse(id)
		if err != nil {
			slog.Error("invalid UUID", "uuid", id, "error", err.Error(), "index", idx)
			return nil, err
		}
		uuids = append(uuids, res)
	}
	slog.Info("All UUIDs are valid")
	return uuids, nil
}

func ProcessUUIDInput(a *App, input []string) error {
	uuids, err := ValidateUUIDs(input)
	if err != nil {
		return err
	}
	var setters []*models.AipSetter
	for _, id := range uuids {
		setters = append(setters, &models.AipSetter{
			UUID:   omit.From(id.String()),
			Status: omit.From(string(AIPStatusNew)),
		})
	}
	_, err = models.Aips.UpsertMany(context.Background(), a.DB, false, []string{"uuid"}, nil, setters...)
	if err != nil {
		return err
	}
	return nil
}

func baseDir(s string) string {
	return s[0:5] // Comes in the form 3cdd/80e1/1d12/4486/acc8/830a/0eb0/9706/
}

func PanicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func formatBool(b bool) string {
	if b {
		return "Done"
	}
	return "Not Done"
}

func CheckSSConnection(a *App) error {
	ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
	location, err := ssAPI.Location.Get(a.Config.LocationUUID)
	if err != nil {
		return fmt.Errorf("error connecting with the SS: %w", err)
	}
	slog.Info("Location for migration", "Description", location.Description, "Path", location.Path)
	return nil
}

func FormatByteSize(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

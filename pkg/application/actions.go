package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/cenkalti/backoff/v4"

	"github.com/artefactual-labs/migrate/pkg/database/gen/models"
	"github.com/artefactual-labs/migrate/pkg/storage_service"
)

type Action struct {
	name string
}

func (a Action) String() string {
	return a.name
}

var (
	ActionFind      = Action{"find"}
	ActionFixity    = Action{"fixity"}
	ActionMove      = Action{"move"}
	ActionReplicate = Action{"Replicate"}
	ActionClean     = Action{"clean"}
	ActionIndex     = Action{"index"}
)

func find(ctx context.Context, a *App, aips ...*models.Aip) error {
	ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
	slog.Info(fmt.Sprintf("Finding %d AIPS", len(aips)))
	for _, aip := range aips {
		e := StartEvent(ActionFind)
		ssPackage, err := ssAPI.Packages.GetByID(ctx, aip.UUID)
		if err != nil {
			if errors.Is(err, storage_service.ErrNotFound) {
				EndEventErr(ctx, a, e, aip, "AIP not found in Storage Service")
				a.UpdateAIPStatus(ctx, aip.ID, AIPStatusNotFound)
				continue
			}
			return err
		}

		slog.Info("AIP found", "UUID", ssPackage.UUID)
		sizeVal := omitnull.Val[int64]{}
		if ssPackage.Size > math.MaxInt64 {
			slog.Warn("package size exceeds supported range", "uuid", ssPackage.UUID, "size", ssPackage.Size)
		} else {
			sizeVal = omitnull.From(int64(ssPackage.Size))
		}

		if ssPackage.Status == "Deleted" {
			a.UpdateAIP(ctx, aip.ID,
				&models.AipSetter{
					Found: omit.From(false),
					Size:  sizeVal,
				},
			)
			EndEvent(ctx, AIPStatusDeleted, a, e, aip)
		} else {
			a.UpdateAIP(ctx, aip.ID,
				&models.AipSetter{
					Found: omit.From(true),
					Size:  sizeVal,
				},
			)
			EndEvent(ctx, AIPStatusFound, a, e, aip)
		}
	}
	return nil
}

func move(ctx context.Context, a *App, aips ...*models.Aip) error {
	ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
	for _, aip := range aips {
		e := StartEvent(ActionMove)
		e.AddDetail(fmt.Sprintf("Moving: %s", aip.UUID))
		if aip.Moved {
			e.AddDetail(fmt.Sprintf("AIP already moved: %s", aip.UUID))
			continue
		}

		ssPackage, err := ssAPI.Packages.GetByID(ctx, aip.UUID)
		if err != nil {
			continue
		}
		if strings.Contains(ssPackage.CurrentLocation, a.Config.MoveLocationUUID) && ssPackage.Status == "UPLOADED" {
			e.AddDetail("AIP already in the desired location")
			EndEvent(ctx, AIPStatusMoved, a, e, aip)
			continue
		}

		if aip.Status == string(AIPStatusMoving) {
			slog.Info("AIP last know Status: moving")
		} else {
			err = ssAPI.Packages.Move(ctx, aip.UUID, a.Config.MoveLocationUUID)
			if err != nil {
				EndEventErr(ctx, a, e, aip, "MOVE operation failed: "+err.Error())
				continue
			}
		}

		moving := true
		b := backoff.NewExponentialBackOff(
			backoff.WithMaxElapsedTime(24*time.Hour),
			backoff.WithMaxInterval(2*time.Minute),
		)
		for moving {
			ssPackage, err = ssAPI.Packages.GetByID(ctx, aip.UUID)
			if err != nil {
				EndEventErr(ctx, a, e, aip, err.Error())
				return err
			}
			if ssPackage.Status == "MOVING" {
				a.UpdateAIPStatus(ctx, aip.ID, AIPStatusMoving)
			} else if ssPackage.Status == "UPLOADED" && strings.Contains(ssPackage.CurrentLocation, a.Config.LocationUUID) {
				a.UpdateAIP(ctx, aip.ID, &models.AipSetter{
					CurrentLocation: omitnull.From(ssPackage.CurrentLocation),
				})
				EndEvent(ctx, AIPStatusMoved, a, e, aip)
				b.Reset()
				moving = false
				continue
			} else {
				err := errors.New("Unexpected AIP Status: " + ssPackage.Status)
				EndEventErr(ctx, a, e, aip, err.Error())
				return err
			}
			timeBackOff := b.NextBackOff()
			slog.Info("Will check again in: " + timeBackOff.String())
			time.Sleep(timeBackOff)
		}
	}
	return nil
}

// func clean(a *App, aips ...*models.Aip) error {
// 	for _, aip := range aips {
// 		e := StartEvent(ActionClean)
// 		e.AddDetail("cleaning local copy: " + aip.LocalCopyPath.MustGet())
//
// 		err := os.RemoveAll(aip.LocalCopyPath.MustGet())
// 		if err != nil {
// 			EndEventErr(a, e, aip, "error cleaning local copy: "+err.Error())
// 			continue
// 		}
//
// 		if a.Config.StagingPath == "" {
// 			e.AddDetail("skipping the cleaning of the staging copy")
// 			EndEvent(StatusCleaned, a, e, aip)
// 			continue
// 		}
//
// 		e.AddDetail("cleaning staging copy: " + aip.StagedCopyPath.MustGet())
// 		err = os.RemoveAll(aip.StagedCopyPath.MustGet())
// 		if err != nil {
// 			EndEventErr(a, e, aip, "error cleaning staging copy: "+err.Error())
// 			continue
// 		}
// 		EndEvent(StatusCleaned, a, e, aip)
// 	}
// 	return nil
// }
//
// func Replicate(a *App, checkReplicas bool, aips ...*models.Aip) error {
// 	for _, aip := range aips {
// 		e := StartEvent(ActionReplicate)
// 		ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
// 		ssPackage, err := ssAPI.Packages.GetByID(aip.UUID)
// 		if err != nil {
// 			return err
// 		}
// 		if checkReplicas && len(ssPackage.Replicas) > 0 {
// 			e.AddDetail("Replica for AIP already exists: " + ssPackage.Replicas[0])
// 			EndEvent(AIPStatusReplicated, a, e, aip)
// 			continue
// 		}
// 		e.AddDetail(fmt.Sprintf("Number of replicas: %d", len(ssPackage.Replicas)))
//
// 		var cmd *exec.Cmd
// 		if a.Config.Docker {
// 			cmd = exec.Command(
// 				"docker",
// 				"exec",
// 				a.Config.SSContainerName,
// 				a.Config.SSManagePath,
// 				"create_aip_replicas",
// 				"--aip-uuid", aip.UUID,
// 				"--replicator-location", a.Config.ReplicationLocationUUID,
// 			)
// 		} else {
// 			cmd = exec.Command(
// 				a.Config.PythonPath,
// 				a.Config.SSManagePath,
// 				"create_aip_replicas",
// 				"--aip-uuid", aip.UUID,
// 				"--replicator-location", a.Config.ReplicationLocationUUID,
// 			)
// 			cmd.Env = cmd.Environ()
// 			cmd.Env = append(cmd.Env,
// 				"DJANGO_SETTINGS_MODULE="+a.Config.DjangoSettingsModule,
// 				"DJANGO_SECRET_KEY="+a.Config.DjangoSecretKey,
// 				"DJANGO_ALLOWED_HOSTS="+a.Config.DjangoAllowedHosts,
// 				"SS_GUNICORN_BIND="+a.Config.SsGunicornBind,
// 				"EMAIL_HOST="+a.Config.EmailHost,
// 				"SS_AUDIT_LOG_MIDDLEWARE="+a.Config.SsAuditLogMiddleware,
// 				"SS_DB_URL="+a.Config.SsDbUrl,
// 				"EMAIL_USE_TLS="+a.Config.EmailUseTls,
// 				"SS_PROMETHEUS_ENABLED="+a.Config.SsPrometheusEnabled,
// 				"DEFAULT_FROM_EMAIL="+a.Config.DefaultFromEmail,
// 				"TIME_ZONE="+a.Config.TimeZone,
// 				"SS_GUNICORN_WORKERS="+a.Config.SsGunicornWorkers,
// 				"REQUESTS_CA_BUNDLE="+a.Config.RequestsCaBundle,
// 			)
// 		}
//
// 		slog.Info("Replicating AIP", "command", cmd.String())
// 		if output, err := cmd.CombinedOutput(); err != nil {
// 			e.AddDetail(string(output))
// 			e.AddDetail(err.Error())
// 			EndEventErr(a, e, aip, err.Error())
// 		} else {
// 			e.AddDetail(string(output))
// 			res := strings.Split(string(output), "\n")
// 			if len(res) > 0 {
// 				sentence := res[len(res)-2]
// 				e.AddDetail("Sentence: " + sentence)
// 				if strings.Contains(sentence, "New replicas created for 1 of 1 AIPs in location") {
// 					EndEvent(AIPStatusReplicated, a, e, aip)
// 					ssPackage, err := ssAPI.Packages.GetByID(aip.UUID)
// 					if err != nil {
// 						return err
// 					}
// 					// Find replica uuid
// 					if len(ssPackage.Replicas) > 0 {
// 						resourceURL := ssPackage.Replicas[0]
// 						split := strings.Split(resourceURL, "/")
// 						uuid := split[len(split)-2]
// 						err = models.Aips.Update(
// 							context.Background(),
// 							a.DB,
// 							&models.AipSetter{ReplicaPath: omitnull.From(uuid)},
// 							&models.Aip{ID: aip.ID},
// 						)
// 						if err != nil {
// 							slog.Error(err.Error())
// 						}
// 					}
//
// 				} else if strings.Contains(sentence, "New replicas created for 0 of 1 AIPs in location.") {
// 					e.AddDetail("Not replicated")
// 					EndEventErr(a, e, aip, sentence)
// 				}
// 			} else {
// 				slog.Info("Replication command returned", "output", string(output))
// 				EndEventErr(a, e, aip, "Could not determine result of Replication")
// 			}
// 		}
// 	}
// 	return nil
// }
//
// func reindex(a *App, aips ...*models.Aip) error {
// 	ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
// 	els := elastic.New(a.Config.ElasticSearchURL)
// 	for _, aip := range aips {
// 		e := StartEvent(ActionIndex)
// 		index, err := els.GetAIPIndex(aip.UUID)
// 		if err != nil {
// 			e.AddDetail(err.Error())
// 			EndEventNoChange(a, e, aip)
// 			return err
// 		}
// 		location, err := ssAPI.Location.Get(aip.NewLocationUUID.MustGet())
// 		if err != nil {
// 			e.AddDetail(err.Error())
// 			EndEventNoChange(a, e, aip)
// 			return err
// 		}
// 		err = els.UpdateAIPIndex(index.ID, aip.NewFullPath.MustGet(), location.Description)
// 		if err != nil {
// 			e.AddDetail(err.Error())
// 			EndEventNoChange(a, e, aip)
// 			return err
// 		}
// 		e.AddDetail("Index update succeeded")
// 		e.AddDetail("AIP new path: " + aip.NewFullPath.MustGet())
// 		e.AddDetail("AIP location description: " + location.Description)
// 		EndEvent(StatusIndexed, a, e, aip)
// 	}
// 	return nil
// }
//
// func checkFixity(a *App, aips ...*models.Aip) error {
// 	ssAPI := storage_service.NewAPI(a.Config.SSURL, a.Config.SSUserName, a.Config.SSAPIKey)
// 	for _, aip := range aips {
// 		e := StartEvent(ActionFixity)
// 		e.AddDetail(fmt.Sprintf("Running fixity for: %s", aip.UUID))
// 		res, err := ssAPI.Packages.CheckFixity(aip.UUID)
// 		if err != nil {
// 			var SSErr storage_service.SSError
// 			if errors.As(err, &SSErr) {
// 				if SSErr.StatusCode >= 500 {
// 					EndEventErrNoFailure(a, e, aip, err.Error())
// 					return errors.New("the storage service has a catastrophic error")
// 				}
// 			}
// 			EndEventErrNoFailure(a, e, aip, err.Error())
// 			continue
// 		}
// 		if res.Success {
// 			EndEvent(StatusFixityChecked, a, e, aip)
// 		} else {
// 			a.AddAIPError(aip, res.Message)
// 			for _, f := range res.Failures.Files.Changed {
// 				e.AddDetail("file changed: " + f)
// 			}
// 			for _, f := range res.Failures.Files.Untracked {
// 				e.AddDetail("file untracked: " + f)
// 			}
// 			for _, f := range res.Failures.Files.Missing {
// 				e.AddDetail("file missing: " + f)
// 			}
// 			EndEventErr(a, e, aip, res.Message)
// 		}
// 	}
// 	return nil
// }

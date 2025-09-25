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
	ActionMove      = Action{"move"}
	ActionReplicate = Action{"Replicate"}
	ActionIndex     = Action{"index"}
)

// find checks if the AIPs exist in the Storage Service and updates their status
// accordingly.
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

// move moves the AIPs to the desired location and updates their status
// accordingly.
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

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

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/storage_service"
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

// move moves the AIPs to the desired location and updates their status
// accordingly.
func move(ctx context.Context, logger *slog.Logger, a *App, storageClient *storage_service.API, aips ...*models.Aip) error {
	for _, aip := range aips {
		e := StartEvent(ActionMove)
		e.AddDetail(fmt.Sprintf("Moving: %s", aip.UUID))
		if aip.Moved {
			e.AddDetail(fmt.Sprintf("AIP already moved: %s", aip.UUID))
			continue
		}

		ssPackage, err := storageClient.Packages.GetByID(ctx, aip.UUID)
		if err != nil {
			continue
		}
		if strings.Contains(ssPackage.CurrentLocation, a.Config.MoveLocationUUID) && ssPackage.Status == "UPLOADED" {
			e.AddDetail("AIP already in the desired location")
			if err := EndEvent(ctx, AIPStatusMoved, a, e, aip); err != nil {
				return err
			}
			continue
		}

		if aip.Status == string(AIPStatusMoving) {
			logger.Info("AIP last know Status: moving")
		} else {
			err = storageClient.Packages.Move(ctx, aip.UUID, a.Config.MoveLocationUUID)
			if err != nil {
				if eventErr := EndEventErr(ctx, a, e, aip, "MOVE operation failed: "+err.Error()); eventErr != nil {
					return eventErr
				}
				continue
			}
		}

		moving := true
		b := backoff.NewExponentialBackOff(
			backoff.WithMaxElapsedTime(24*time.Hour),
			backoff.WithMaxInterval(2*time.Minute),
		)
		for moving {
			ssPackage, err = storageClient.Packages.GetByID(ctx, aip.UUID)
			if err != nil {
				if eventErr := EndEventErr(ctx, a, e, aip, err.Error()); eventErr != nil {
					return eventErr
				}
				return err
			}
			if ssPackage.Status == "MOVING" {
				if err := a.UpdateAIPStatus(ctx, aip.ID, AIPStatusMoving); err != nil {
					return err
				}
			} else if ssPackage.Status == "UPLOADED" && strings.Contains(ssPackage.CurrentLocation, a.Config.LocationUUID) {
				if err := a.UpdateAIP(ctx, aip.ID, &models.AipSetter{
					CurrentLocation: omitnull.From(ssPackage.CurrentLocation),
				}); err != nil {
					return err
				}
				if err := EndEvent(ctx, AIPStatusMoved, a, e, aip); err != nil {
					return err
				}
				b.Reset()
				moving = false
				continue
			} else {
				err := errors.New("Unexpected AIP Status: " + ssPackage.Status)
				if eventErr := EndEventErr(ctx, a, e, aip, err.Error()); eventErr != nil {
					return eventErr
				}
				return err
			}
			timeBackOff := b.NextBackOff()
			logger.Info("Will check again in: " + timeBackOff.String())
			time.Sleep(timeBackOff)
		}
	}
	return nil
}

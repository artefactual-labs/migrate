package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aarondl/opt/omitnull"
	"github.com/cenkalti/backoff/v4"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

const MoveActivityName = "Move Activity"

type MoveActivityParams struct {
	UUID string `json:"uuid"`
}
type MoveActivityResult struct {
	Status string
}

func (a *App) MoveA(ctx context.Context, params MoveActivityParams) (*MoveActivityResult, error) {
	aip, err := a.GetAIPByID(ctx, params.UUID)
	if err != nil {
		return nil, err
	}
	err = move(ctx, a.logger, a, a.StorageClient, aip)
	if err != nil {
		return nil, err
	}
	if err := aip.Reload(ctx, a.DB); err != nil {
		return nil, err
	}
	result := &MoveActivityResult{
		Status: aip.Status,
	}
	return result, nil
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
			} else if ssPackage.Status == "UPLOADED" && strings.Contains(ssPackage.CurrentLocation, a.Config.MoveLocationUUID) {
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

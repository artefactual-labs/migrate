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
		if strings.Contains(ssPackage.CurrentLocation, a.Locations.MoveTargetLocationID) && ssPackage.Status == "UPLOADED" {
			e.AddDetail("AIP already in the desired location")
			if err := EndEvent(ctx, AIPStatusMoved, a, e, aip); err != nil {
				return err
			}
			continue
		}

		if aip.Status == string(AIPStatusMoving) {
			logger.Info("AIP last know Status: moving")
		} else {
			err = storageClient.Packages.Move(ctx, aip.UUID, a.Locations.MoveTargetLocationID)
			if err != nil {
				if eventErr := EndEventErr(ctx, a, e, aip, "MOVE operation failed: "+err.Error()); eventErr != nil {
					return eventErr
				}
				continue
			}
		}

		// We use backoff to control the wait time between polling attempts
		// (starting at ~500ms, growing by 1.5× each time, capping at 10 mins)
		// for up to an overall maximum of 24 hours, until either:
		// - The move completes successfully, or
		// - The backoff is exhausted (24h total), or
		// - An error occurs.
		backoffStrategy := backoff.NewExponentialBackOff(
			backoff.WithMaxElapsedTime(24*time.Hour),
			backoff.WithMaxInterval(10*time.Minute),
		)

		moving := true
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
			} else if ssPackage.Status == "UPLOADED" && strings.Contains(ssPackage.CurrentLocation, a.Locations.MoveTargetLocationID) {
				if err := a.UpdateAIP(ctx, aip.ID, &models.AipSetter{
					CurrentLocation: omitnull.From(ssPackage.CurrentLocation),
				}); err != nil {
					return err
				}
				if err := EndEvent(ctx, AIPStatusMoved, a, e, aip); err != nil {
					return err
				}
				moving = false
				continue
			} else {
				err := errors.New("Unexpected AIP Status: " + ssPackage.Status)
				if eventErr := EndEventErr(ctx, a, e, aip, err.Error()); eventErr != nil {
					return eventErr
				}
				return err
			}

			// Wait for the next backoff interval before polling again.
			if timeBackOff := backoffStrategy.NextBackOff(); timeBackOff == backoff.Stop {
				err := errors.New("move polling backoff exhausted")
				logger.Warn("Backoff exhausted, aborting move polling.", slog.String("aip", aip.UUID))
				if eventErr := EndEventErr(ctx, a, e, aip, err.Error()); eventErr != nil {
					return eventErr
				}
				return err
			} else {
				// Note: this is not too noisy (~160 entries over 24h).
				logger.Warn("Will check again in: " + timeBackOff.String())
				time.Sleep(timeBackOff)
			}
		}
	}
	return nil
}

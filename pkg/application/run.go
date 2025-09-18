package application

import (
	"context"
	"sort"
	"time"

	"github.com/artefactual-labs/migrate/pkg/database/gen/models"
)

// func (a *App) RunDaemonOne(input []string) error {
// 	if err := CheckSSConnection(a); err != nil {
// 		return err
// 	}
// 	if err := ProcessUUIDInput(a, input); err != nil {
// 		return err
// 	}
//
// 	aips, err := a.GetAIPsByStatus(StatusNew)
// 	if err != nil {
// 		return err
// 	}
// 	if err := find(a, aips...); err != nil {
// 		return err
// 	}
//
// 	aips, err = a.GetAIPsByStatus(
// 		StatusFound,
// 		StatusFixityChecked,
// 		StatusMoving,
// 		StatusMoved,
// 		StatusCleaned,
// 		AIPStatusReplicated,
// 		StatusIndexed,
// 	)
// 	if err != nil {
// 		return err
// 	}
//
// 	// Will begin migrating the smallest AIPs first.
// 	SortAips(aips)
//
// 	for _, aip := range aips {
// 		if a.Config.CheckFixity && !aip.FixityRun {
// 			if err := checkFixity(a, aip); err != nil {
// 				return err
// 			}
// 			a.reloadAIP(aip)
// 		}
//
// 		if a.Config.Move && !aip.Moved {
// 			if err := move(a, aip); err != nil {
// 				return err
// 			}
// 			a.reloadAIP(aip)
// 		}
//
// 		if a.Config.Replicate && !aip.Replicated {
// 			if aip.Status != string(StatusMoved) {
// 				continue
// 			}
// 			if err := Replicate(a, true, aip); err != nil {
// 				return err
// 			}
// 			a.reloadAIP(aip)
// 		}
//
// 		if a.Config.Clean && !aip.Cleaned {
// 			if aip.Status != string(AIPStatusReplicated) {
// 				continue
// 			}
// 			if err := clean(a, aip); err != nil {
// 				return err
// 			}
// 			a.reloadAIP(aip)
// 		}
//
// 		if a.Config.ReIndex && !aip.ReIndexed {
// 			if err := reindex(a, aip); err != nil {
// 				return err
// 			}
// 			a.reloadAIP(aip)
// 		}
//
// 		if IsComplete(aip) {
// 			a.UpdateAIPStatus(aip.ID, AIPStatusFinished)
// 		}
//
// 		calculateDuration(a, aip)
// 		a.reloadAIP(aip)
// 		slog.Info("Total duration: " + aip.TotalDuration.GetOrZero())
// 	}
//
// 	return nil
// }

func calculateDuration(a *App, aip *models.Aip) {
	a.reloadAIP(aip)
	err := aip.LoadAipEvents(context.Background(), a.DB)
	PanicIfErr(err)

	var durationNanoSeconds int64
	for _, e := range aip.R.Events {
		durationNanoSeconds += e.TotalDurationNanoseconds.GetOrZero()
	}
	_ = time.Duration(durationNanoSeconds)
	// err = aip.Update(context.Background(), a.DB, &models.AipSetter{
	// 	TotalDuration:           omitnull.From(d.String()),
	// 	TotalDurationNanosecond: omitnull.From(uint64(d.Nanoseconds())),
	// })
	PanicIfErr(err)
}

func (a *App) reloadAIP(aip *models.Aip) {
	err := aip.Reload(context.Background(), a.DB)
	PanicIfErr(err)
}

func IsComplete(aip *models.Aip) bool {
	return aip.FixityRun && aip.Moved && aip.Cleaned && aip.Replicated && aip.ReIndexed
}

func SortAips(aips []*models.Aip) {
	sort.Slice(aips, func(i, j int) bool {
		return aips[i].Size.GetOrZero() < aips[j].Size.GetOrZero()
	})
}

package application

import (
	"sort"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
)

func IsComplete(aip *models.Aip) bool {
	return aip.FixityRun && aip.Moved && aip.Cleaned && aip.Replicated && aip.ReIndexed
}

func SortAips(aips []*models.Aip) {
	sort.Slice(aips, func(i, j int) bool {
		return aips[i].Size.GetOrZero() < aips[j].Size.GetOrZero()
	})
}

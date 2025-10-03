package application

import (
	"sort"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
)

func SortAips(aips []*models.Aip) {
	sort.Slice(aips, func(i, j int) bool {
		return aips[i].Size.GetOrZero() < aips[j].Size.GetOrZero()
	})
}

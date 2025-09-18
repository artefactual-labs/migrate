package application

import "context"

// func (a *App) Replicate(input []string) error {
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
// 	aips, err = a.GetAIPsByStatus(StatusFound)
// 	if err != nil {
// 		return err
// 	}
//
// 	// Will begin migrating the smallest AIPs first.
// 	SortAips(aips)
//
// 	err = Replicate(a, false, aips...)
// 	return err
// }

func (a *App) Move(ctx context.Context, input []string) error {
	if err := CheckSSConnection(ctx, a); err != nil {
		return err
	}
	if err := ProcessUUIDInput(ctx, a, input); err != nil {
		return err
	}

	aips, err := a.GetAIPsByStatus(ctx, AIPStatusNew)
	if err != nil {
		return err
	}
	if err := find(ctx, a, aips...); err != nil {
		return err
	}

	aips, err = a.GetAIPsByStatus(ctx, AIPStatusFound)
	if err != nil {
		return err
	}

	err = move(ctx, a, aips...)
	return err
}

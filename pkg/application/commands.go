package application

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

func (a *App) Move(input []string) error {
	if err := CheckSSConnection(a); err != nil {
		return err
	}
	if err := ProcessUUIDInput(a, input); err != nil {
		return err
	}

	aips, err := a.GetAIPsByStatus(AIPStatusNew)
	if err != nil {
		return err
	}
	if err := find(a, aips...); err != nil {
		return err
	}

	aips, err = a.GetAIPsByStatus(AIPStatusFound)
	if err != nil {
		return err
	}

	err = move(a, aips...)
	return err
}

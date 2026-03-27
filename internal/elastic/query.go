package elastic

type QueryAIPUUID struct {
	Query struct {
		Term struct {
			UUID string `json:"uuid"`
		} `json:"term"`
	} `json:"query"`
}

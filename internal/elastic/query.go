package elastic

type QueryAIPUUID_V6 struct {
	Query struct {
		Term struct {
			UUID string `json:"uuid"`
		} `json:"term"`
	} `json:"query"`
}

type QueryAIPUUID_V8 struct {
	Query struct {
		Term struct {
			UUID struct {
				Value string `json:"value"`
			} `json:"uuid"`
		} `json:"term"`
	} `json:"query"`
}

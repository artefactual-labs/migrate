package elastic

type ElasticAipIndexResponse struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		MaxScore float64 `json:"max_score"`
		Hits     []struct {
			Index  string  `json:"_index"`
			ID     string  `json:"_id"`
			Score  float64 `json:"_score"`
			Source struct {
				UUID             string   `json:"uuid"`
				Name             string   `json:"name"`
				FilePath         string   `json:"filePath"`
				Size             float64  `json:"size"`
				FileCount        int      `json:"file_count"`
				Origin           string   `json:"origin"`
				Created          int      `json:"created"`
				Aicid            any      `json:"AICID"`
				IsPartOf         string   `json:"isPartOf"`
				CountAIPsinAIC   any      `json:"countAIPsinAIC"`
				Identifiers      []any    `json:"identifiers"`
				TransferMetadata []any    `json:"transferMetadata"`
				Encrypted        bool     `json:"encrypted"`
				Accessionids     []string `json:"accessionids"`
				Status           string   `json:"status"`
				Location         string   `json:"location"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

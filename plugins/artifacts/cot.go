package artifacts

type cotArtifact struct {
	Sha256 string `json:"sha256"`
}

type chainOfTrust struct {
	Version     int                    `json:"chainOfTrustVersion"`
	TaskID      string                 `json:"taskId"`
	RunID       int                    `json:"runId"`
	WorkerGroup string                 `json:"workerGroup"`
	WorkerID    string                 `json:"workerId"`
	Environment map[string]interface{} `json:"environment"`
	Task        interface{}            `json:"task"`
	Artifacts   map[string]cotArtifact `json:"artifacts"`
}

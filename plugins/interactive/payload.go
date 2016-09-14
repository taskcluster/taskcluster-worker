package interactive

type payload struct {
	Interactive *opts `json:"interactive,omitempty"`
}

type opts struct {
	ArtifactPrefix string `json:"artifactPrefix"`
	DisableDisplay bool   `json:"disableDisplay"`
	DisableShell   bool   `json:"disableShell"`
}

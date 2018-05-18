package imagecache

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/taskcluster/taskcluster-worker/runtime/caching"
)

// Messages from the docker pull JSON stream
type dockerPullMessage struct {
	Stream   string `json:"stream,omitempty"`
	Status   string `json:"status,omitempty"`
	Progress *struct {
		Current int64 `json:"current,omitempty"`
		Total   int64 `json:"total,omitempty"`
		Start   int64 `json:"start,omitempty"`
		// If true, don't show xB/yB
		HideCounts bool   `json:"hidecounts,omitempty"`
		Units      string `json:"units,omitempty"`
	} `json:"progressDetail,omitempty"`
	ID       string `json:"id,omitempty"`
	From     string `json:"from,omitempty"`
	Time     int64  `json:"time,omitempty"`
	TimeNano int64  `json:"timeNano,omitempty"`
	Error    *struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"errorDetail,omitempty"`
}

// reportDockerPullProgress reads JSON messags from docker and sends progress
// updates to the ctx
func reportDockerPullProgress(ctx caching.Context, r io.Reader) {
	dec := json.NewDecoder(r)
	lastProgress := make(map[string]float64) // track last progress reporting for given description
	for {
		// Read message from the stream
		var m dockerPullMessage
		if err := dec.Decode(&m); err != nil {
			break // We're just going to ignore the error, it'll be reported else where
		}

		// If we have an error, we pretty much have to report that
		if m.Error != nil {
			if m.ID != "" {
				ctx.Progress(fmt.Sprintf("%s - %s", m.ID, m.Error.Message), 1)
			} else {
				ctx.Progress(m.Error.Message, 1)
			}
			continue
		}

		// If we don't have ID or progress, let's skip reporting anything
		if m.ID == "" || m.Progress == nil || m.Progress.Total == 0 {
			continue
		}
		progress := float64(m.Progress.Current) / float64(m.Progress.Total)
		progress = float64(int(progress*10)) / 10 // cut to one decimal
		if p, ok := lastProgress[m.Status+m.ID]; !ok || p != progress {
			// We only report progress for each ID for every 10% which is a 0.1 increment
			lastProgress[m.Status+m.ID] = progress
			ctx.Progress(fmt.Sprintf("%s - %s", m.Status, m.ID), progress)
		}
	}
}

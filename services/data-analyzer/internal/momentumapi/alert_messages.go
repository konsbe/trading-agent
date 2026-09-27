package momentumapi

import (
	"encoding/json"
	"fmt"
	"os"
)

// AlertMessages is shared/content/alert_messages.json: each alert type's
// display label and message template. The analyst bot renders the template
// into fired_alerts.message; momentum-api serves the labels (GET /alerts
// type_labels), so Discord and Alarm History name an alert the same way. No
// fallback: a missing or incomplete file stops the server at start.
type AlertMessages struct {
	Types map[string]struct {
		Label   string `json:"label"`
		Message string `json:"message"`
	} `json:"alert_types"`
}

func LoadAlertMessages(path string) (AlertMessages, error) {
	var m AlertMessages
	b, err := os.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("alert messages: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("alert messages %s: %w", path, err)
	}
	if len(m.Types) == 0 {
		return m, fmt.Errorf("alert messages %s: alert_types is missing or empty", path)
	}
	for k, t := range m.Types {
		if t.Label == "" || t.Message == "" {
			return m, fmt.Errorf("alert messages %s: %s lacks a label or message", path, k)
		}
	}
	return m, nil
}

// Labels maps each alert type to its display label.
func (m AlertMessages) Labels() map[string]string {
	out := make(map[string]string, len(m.Types))
	for k, t := range m.Types {
		out[k] = t.Label
	}
	return out
}

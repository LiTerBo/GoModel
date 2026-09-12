package auditlog

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const virtualModelEventProvider = "virtual_model"

// Management actions recorded for a virtual model. Retarget covers any change to
// what the row resolves to; Rename covers a change of the name callers address.
const (
	VirtualModelActionRetarget = "retarget"
	VirtualModelActionRename   = "rename"
	VirtualModelActionLock     = "lock"
	VirtualModelActionUnlock   = "unlock"
	VirtualModelActionDelete   = "delete"
)

// VirtualModelChange is one management action on a virtual model. The counts are
// the credentials and user paths the row reached at the time, recorded with every
// action so the trail answers "who does this row serve" without replaying the
// policies of that day.
type VirtualModelChange struct {
	Timestamp      time.Time
	Action         string
	Source         string
	PreviousSource string
	OldTargets     []string
	NewTargets     []string
	Locked         bool
	Forced         bool
	Credentials    int
	UserPaths      int
	Method         string
	Path           string
	ClientIP       string
	UserAgent      string
	RequestID      string
}

// VirtualModelChangeSnapshot carries the change itself inside the audit entry.
type VirtualModelChangeSnapshot struct {
	Action         string   `json:"action" bson:"action"`
	Source         string   `json:"source" bson:"source"`
	PreviousSource string   `json:"previous_source,omitempty" bson:"previous_source,omitempty"`
	OldTargets     []string `json:"old_targets,omitempty" bson:"old_targets,omitempty"`
	NewTargets     []string `json:"new_targets,omitempty" bson:"new_targets,omitempty"`
	Locked         bool     `json:"locked,omitempty" bson:"locked,omitempty"`
	Forced         bool     `json:"forced,omitempty" bson:"forced,omitempty"`
	Credentials    int      `json:"credentials,omitempty" bson:"credentials,omitempty"`
	UserPaths      int      `json:"user_paths,omitempty" bson:"user_paths,omitempty"`
}

// VirtualModelChangeEntry maps one management action onto a durable audit entry
// in the same trail as requests and authentication events, so the console's
// existing audit queries reach it (provider "virtual_model").
func VirtualModelChangeEntry(change VirtualModelChange) *LogEntry {
	timestamp := change.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	return &LogEntry{
		ID:         uuid.NewString(),
		Timestamp:  timestamp.UTC(),
		Provider:   virtualModelEventProvider,
		StatusCode: http.StatusOK,
		RequestID:  strings.TrimSpace(change.RequestID),
		ClientIP:   strings.TrimSpace(change.ClientIP),
		Method:     strings.TrimSpace(change.Method),
		Path:       strings.TrimSpace(change.Path),
		Data: &LogData{
			UserAgent: strings.TrimSpace(change.UserAgent),
			EventType: virtualModelEventType(change.Action),
			VirtualModel: &VirtualModelChangeSnapshot{
				Action:         change.Action,
				Source:         strings.TrimSpace(change.Source),
				PreviousSource: strings.TrimSpace(change.PreviousSource),
				OldTargets:     change.OldTargets,
				NewTargets:     change.NewTargets,
				Locked:         change.Locked,
				Forced:         change.Forced,
				Credentials:    change.Credentials,
				UserPaths:      change.UserPaths,
			},
		},
	}
}

// virtualModelEventType is the lifecycle event id for one action. An action this
// build does not know is still recorded, marked unknown rather than guessed.
func virtualModelEventType(action string) string {
	switch action {
	case VirtualModelActionRetarget, VirtualModelActionRename, VirtualModelActionLock,
		VirtualModelActionUnlock, VirtualModelActionDelete:
		return virtualModelEventProvider + "_" + action
	default:
		return virtualModelEventProvider + "_unknown"
	}
}

// NewVirtualModelChangeSink returns the sink the admin layer calls after a
// management action. It writes through the same logger, retention, storage
// backend and shutdown flush as request auditing, and does nothing while
// auditing is disabled.
func NewVirtualModelChangeSink(logger LoggerInterface) func(VirtualModelChange) {
	return func(change VirtualModelChange) {
		if logger == nil || !logger.Config().Enabled {
			return
		}
		logger.Write(VirtualModelChangeEntry(change))
	}
}

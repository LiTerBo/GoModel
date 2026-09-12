package admin

import (
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// T13: management actions on a virtual model are recorded in the audit trail, so
// an operator can answer later what changed, and who the change reached, without
// replaying that day's policies. The sink is optional: without one the actions
// still happen, they are just not recorded.

// WithVirtualModelEvents installs the sink that receives virtual model
// management actions. The composition root wires it to the audit log.
func WithVirtualModelEvents(sink func(auditlog.VirtualModelChange)) Option {
	return func(h *Handler) {
		h.virtualModelEvents = sink
	}
}

// emitVirtualModelChange reports one management action. Actor identity is not
// available to the admin layer: requests authenticate with the master key or a
// dashboard credential and nothing carries that identity into the handler, so
// the event records the request metadata that is available instead.
func (h *Handler) emitVirtualModelChange(c *echo.Context, change auditlog.VirtualModelChange) {
	if h.virtualModelEvents == nil {
		return
	}
	if change.Timestamp.IsZero() {
		change.Timestamp = time.Now().UTC()
	}
	if c != nil {
		if request := c.Request(); request != nil {
			change.Method = request.Method
			change.Path = request.URL.Path
			change.ClientIP = request.RemoteAddr
			change.UserAgent = request.UserAgent()
		}
	}
	h.virtualModelEvents(change)
}

// virtualModelChangeAction classifies an upsert for the audit trail, or returns
// "" when the write is not a management action: a rename wins over a retarget
// (the snapshot still carries both target lists), a pointing change is a
// retarget, and a pure lock toggle is recorded as such. Edits that leave both
// the pointing and the lock alone are not recorded, and neither is a creation:
// a new row has nobody following it yet.
func virtualModelChangeAction(stored *virtualmodels.VirtualModel, next virtualmodels.VirtualModel, renamed bool) string {
	if stored == nil {
		return ""
	}
	if renamed {
		return auditlog.VirtualModelActionRename
	}
	if virtualmodels.ResolutionConfigChanged(*stored, next) {
		return auditlog.VirtualModelActionRetarget
	}
	switch {
	case stored.Locked == next.Locked:
		return ""
	case next.Locked:
		return auditlog.VirtualModelActionLock
	default:
		return auditlog.VirtualModelActionUnlock
	}
}

// upsertVirtualModelChange builds the audit event for a successful upsert. It
// reports the reachability of the stored row: those are the callers the change
// reached, whether or not the action kept them on the alias.
func (h *Handler) upsertVirtualModelChange(c *echo.Context, stored *virtualmodels.VirtualModel, next virtualmodels.VirtualModel, action string) {
	if action == "" {
		return
	}
	change := auditlog.VirtualModelChange{
		Action:     action,
		Source:     next.Source,
		Locked:     next.Locked,
		NewTargets: targetStrings(next),
	}
	if stored != nil {
		change.OldTargets = targetStrings(*stored)
		change.PreviousSource = stored.Source
		if action != auditlog.VirtualModelActionRename {
			change.PreviousSource = ""
		}
		used := h.virtualModelUsage(stored)
		change.Credentials = countGrantKind(used, "credential")
		change.UserPaths = countGrantKind(used, "user_path")
	}
	h.emitVirtualModelChange(c, change)
}

// targetStrings renders a row's targets as "provider/model" strings for the
// audit snapshot.
func targetStrings(vm virtualmodels.VirtualModel) []string {
	if len(vm.Targets) == 0 {
		return nil
	}
	out := make([]string, 0, len(vm.Targets))
	for _, target := range vm.Targets {
		label := strings.TrimSpace(target.Provider) + "/" + strings.TrimSpace(target.Model)
		if strings.TrimSpace(target.Provider) == "" {
			label = strings.TrimSpace(target.Model)
		}
		out = append(out, label)
	}
	return out
}

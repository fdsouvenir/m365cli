package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fdsouvenir/m365cli/internal/auth"
	"github.com/fdsouvenir/m365cli/internal/config"
)

func grantReport(record config.ProfileRecord) map[string]any {
	grant := config.GrantRecord{Status: "unknown", Reason: "Grant has not been checked."}
	if record.Grant != nil {
		grant = *record.Grant
	}
	missing, extra := auth.GrantDifferences(record.DelegatedScopeWorkloads, grant)
	return map[string]any{"status": grant.Status, "scopes": grant.Scopes, "checked_at": grant.CheckedAt, "reason": grant.Reason, "selected_not_granted": missing, "granted_not_selected": extra}
}

func profileReport(record config.ProfileRecord) map[string]any {
	// Preserve the pre-2.1 profile fields and their omission behavior.
	b, _ := json.Marshal(record)
	var payload map[string]any
	_ = json.Unmarshal(b, &payload)
	payload["grant"] = grantReport(record)
	return payload
}

func saveCheckedGrant(record config.ProfileRecord) error {
	return config.UpdateConfig(func(cfg *config.File) error {
		current, ok := cfg.Profiles[record.Name]
		if !ok || current.ClientID != record.ClientID || current.AccountID != record.AccountID || current.Authority != record.Authority {
			return fmt.Errorf("profile changed during grant inspection; run auth check again")
		}
		current.Grant = record.Grant
		cfg.Profiles[record.Name] = current
		return nil
	})
}

func printGrant(record config.ProfileRecord) {
	report := grantReport(record)
	fmt.Fprintf(os.Stdout, "    Grant: %s", report["status"])
	if record.Grant != nil && record.Grant.CheckedAt != nil {
		fmt.Fprintf(os.Stdout, " (checked %s)", record.Grant.CheckedAt.Format(time.RFC3339))
	}
	fmt.Fprintln(os.Stdout)
	if report["status"] == "unknown" {
		fmt.Fprintln(os.Stdout, "    "+report["reason"].(string))
		return
	}
	if len(record.DelegatedScopeWorkloads) == 0 {
		fmt.Fprintln(os.Stdout, "    Selection not recorded; grant differences are unknown.")
		return
	}
	missing, extra := auth.GrantDifferences(record.DelegatedScopeWorkloads, *record.Grant)
	fmt.Fprintf(os.Stdout, "    Selected but not granted: %s\n    Granted but not selected: %s\n", displayOrDash(strings.Join(missing, ", ")), displayOrDash(strings.Join(extra, ", ")))
}

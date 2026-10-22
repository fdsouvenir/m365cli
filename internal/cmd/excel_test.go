package cmd

import (
	"context"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/config"
	"github.com/fdsouvenir/m365cli/internal/profile"
)

func TestExcelAccountBoundariesAndPreview(t *testing.T) {
	setTempUserConfigEnv(t)
	for _, tc := range []struct {
		name, audience, mode string
		allowed              bool
	}{
		{"personal", "consumer", "delegated", false},
		{"service", "enterprise", "app_only", false},
		{"work", "enterprise", "delegated", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := profile.NewStore().Upsert(config.ProfileRecord{Name: tc.name, ClientID: "client", Audience: tc.audience, AuthMode: tc.mode}, false); err != nil {
				t.Fatal(err)
			}
			ctx := withRootFlags(context.Background(), &RootFlags{Profile: tc.name})
			// No credentials or transport configured: successful preview proves no API call.
			c := ExcelWorksheetsAddCmd{excelWorkbookFlags: excelWorkbookFlags{Workbook: "book"}, Name: "Report", DryRun: true}
			err := c.Run(ctx)
			if (err == nil) != tc.allowed {
				t.Fatalf("preview error %v, allowed=%v", err, tc.allowed)
			}
		})
	}
}

package auth

// Workload is the public delegated-permission catalog consumed by login and Link.
type Workload struct {
	ID                  string   `json:"id"`
	Service             string   `json:"service"`
	Label               string   `json:"label"`
	Access              string   `json:"access"`
	Scopes              []string `json:"scopes"`
	Personal            bool     `json:"personal"`
	Work                bool     `json:"work"`
	TenantAdminConsent  bool     `json:"tenant_admin_consent"`
	CompatibilityBundle bool     `json:"compatibility_bundle"`
	Notes               string   `json:"notes"`
}

var workloadCatalog = []Workload{
	{"mail", "mail", "Read and send email", "read/write", []string{"Mail.Read", "Mail.Send"}, true, true, false, true, "Read and send only; modifying messages and saving drafts require mail-write."},
	{"calendar", "calendar", "Read and change calendars (compatibility)", "read/write", []string{"Calendars.Read", "Calendars.ReadWrite"}, true, true, false, true, ""},
	{"contacts", "contacts", "Read and change contacts (compatibility)", "read/write", []string{"Contacts.Read", "Contacts.ReadWrite"}, true, true, false, true, ""},
	{"tasks", "tasks", "Read and change To Do tasks (compatibility)", "read/write", []string{"Tasks.Read", "Tasks.ReadWrite"}, true, true, false, true, ""},
	{"onedrive", "onedrive", "Read and change OneDrive files (compatibility)", "read/write", []string{"Files.Read", "Files.ReadWrite"}, true, true, false, true, ""},
	{"groups", "groups", "Read Microsoft 365 Groups and membership", "read", []string{"Group.Read.All", "GroupMember.Read.All"}, false, true, true, false, "Requires a work account and tenant-administrator consent."},
	{"excel", "excel", "Read and change Excel workbooks", "read/write", []string{"Files.ReadWrite"}, false, true, false, false, "Excel cannot be read-only: Microsoft's workbook APIs require Files.ReadWrite even for reads. This grants read and write access to all of the person's OneDrive files, not just workbooks. Business storage and delegated sign-in are required."},
	{"mail-read", "mail", "Read email", "read", []string{"Mail.Read"}, true, true, false, false, ""},
	{"mail-send", "mail", "Send email", "send", []string{"Mail.Send"}, true, true, false, false, ""},
	{"mail-write", "mail", "Read and change email and save drafts", "read/write", []string{"Mail.ReadWrite"}, true, true, false, false, "Does not include sending email."},
	{"calendar-read", "calendar", "Read calendars", "read", []string{"Calendars.Read"}, true, true, false, false, ""},
	{"calendar-write", "calendar", "Read and change calendars", "read/write", []string{"Calendars.ReadWrite"}, true, true, false, false, ""},
	{"contacts-read", "contacts", "Read contacts", "read", []string{"Contacts.Read"}, true, true, false, false, ""},
	{"contacts-write", "contacts", "Read and change contacts", "read/write", []string{"Contacts.ReadWrite"}, true, true, false, false, ""},
	{"tasks-read", "tasks", "Read To Do tasks", "read", []string{"Tasks.Read"}, true, true, false, false, ""},
	{"tasks-write", "tasks", "Read and change To Do tasks", "read/write", []string{"Tasks.ReadWrite"}, true, true, false, false, ""},
	{"onedrive-read", "onedrive", "Read OneDrive files", "read", []string{"Files.Read"}, true, true, false, false, ""},
	{"onedrive-write", "onedrive", "Read and change OneDrive files", "read/write", []string{"Files.ReadWrite"}, true, true, false, false, ""},
}

func DelegatedWorkloads() []Workload {
	out := make([]Workload, len(workloadCatalog))
	for i, w := range workloadCatalog {
		out[i] = w
		out[i].Scopes = append([]string{}, w.Scopes...)
	}
	return out
}

func WorkloadByID(id string) (Workload, bool) {
	for _, w := range workloadCatalog {
		if w.ID == id {
			return w, true
		}
	}
	return Workload{}, false
}

// ScopesCover only applies Graph's same-service ReadWrite -> Read implication.
func ScopesCover(granted, required []string) bool { return scopeSetContainsAll(granted, required) }

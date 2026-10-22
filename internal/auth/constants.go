package auth

var BaseDelegatedScopes = []string{
	"openid",
	"profile",
	"offline_access",
	"User.Read",
}

var delegatedScopeWorkloadOrder = []string{
	"mail",
	"calendar",
	"contacts",
	"tasks",
	"onedrive",
	"groups",
	"excel",
	"mail-read",
	"mail-send",
	"mail-write",
}

var delegatedScopeWorkloadMap = map[string][]string{
	"excel":      {"Files.ReadWrite"},
	"mail-read":  {"Mail.Read"},
	"mail-send":  {"Mail.Send"},
	"mail-write": {"Mail.ReadWrite"},
	"mail": {
		"Mail.Read",
		"Mail.Send",
	},
	"calendar": {
		"Calendars.Read",
		"Calendars.ReadWrite",
	},
	"contacts": {
		"Contacts.Read",
		"Contacts.ReadWrite",
	},
	"tasks": {
		"Tasks.Read",
		"Tasks.ReadWrite",
	},
	"onedrive": {
		"Files.Read",
		"Files.ReadWrite",
	},
	"groups": {
		"Group.Read.All",
		"GroupMember.Read.All",
	},
}

var AllDelegatedWorkloadScopes = []string{
	"Mail.ReadWrite",
	"Mail.Read",
	"Mail.Send",
	"Calendars.Read",
	"Calendars.ReadWrite",
	"Contacts.Read",
	"Contacts.ReadWrite",
	"Tasks.Read",
	"Tasks.ReadWrite",
	"Files.Read",
	"Files.ReadWrite",
	"Group.Read.All",
	"GroupMember.Read.All",
}

package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/graph"
)

func TestListUsesPageTokenURLAndSearchHeader(t *testing.T) {
	t.Parallel()

	requests := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/mail/next" {
			t.Fatalf("expected /mail/next path, got %s", r.URL.Path)
		}
		if r.Header.Get("ConsistencyLevel") != "eventual" {
			t.Fatalf("expected ConsistencyLevel eventual header, got %q", r.Header.Get("ConsistencyLevel"))
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"id":"1"},{"id":"2"}],"@odata.nextLink":"%s/mail/next?state=next"}`, serverURL)
	}))
	defer server.Close()
	serverURL = server.URL

	client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	client.BaseURL = serverURL
	client.HTTPClient = server.Client()

	svc := New(client, "")
	page := serverURL + "/mail/next?%24search=%22foo%22&state=abc"
	items, next, err := svc.List(context.Background(), 1, "", page, "inbox")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if len(items) != 2 {
		t.Fatalf("must retain full server page for lossless continuation, got %d", len(items))
	}
	if next != serverURL+"/mail/next?state=next" {
		t.Fatalf("unexpected next link: %s", next)
	}
}

func TestEndpointsRouteByAuthMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		user             string
		wantListPath     string
		wantGetPath      string
		wantSendMailPath string
	}{
		{
			name:             "delegated uses me endpoints",
			user:             "",
			wantListPath:     "/me/messages",
			wantGetPath:      "/me/messages/message-id",
			wantSendMailPath: "/me/sendMail",
		},
		{
			name:             "app-only uses user endpoints",
			user:             "person@example.com",
			wantListPath:     "/users/" + url.PathEscape("person@example.com") + "/messages",
			wantGetPath:      "/users/" + url.PathEscape("person@example.com") + "/messages/message-id",
			wantSendMailPath: "/users/" + url.PathEscape("person@example.com") + "/sendMail",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			expectedMethod, expectedPath, expectedScope := "", "", ""
			var expectedBody map[string]any
			requests := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if expectedPath != "" {
					if r.Method != expectedMethod || r.URL.Path != expectedPath {
						t.Errorf("wrong action: %s %s; want %s %s", r.Method, r.URL.Path, expectedMethod, expectedPath)
					}
					var body map[string]any
					if expectedBody != nil {
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						if !reflect.DeepEqual(body, expectedBody) {
							t.Errorf("wrong action body: %v want %v", body, expectedBody)
						}
					}
					if r.Method == http.MethodGet {
						if strings.HasSuffix(r.URL.Path, "/mailFolders/deleteditems") {
							fmt.Fprint(w, `{"id":"deleted-id"}`)
							return
						}
						fmt.Fprint(w, `{"value":[{"id":"folder-id"}]}`)
					} else {
						fmt.Fprint(w, `{"id":"result-id"}`)
					}
					return
				}

				switch {
				case r.Method == http.MethodGet && r.URL.Path == tc.wantListPath:
					if got := r.URL.Query().Get("$orderby"); got != "receivedDateTime desc" {
						t.Errorf("newest-first ordering missing: %q", got)
					}
					_, _ = fmt.Fprint(w, `{"value":[]}`)
				case r.Method == http.MethodGet && r.URL.Path == strings.TrimSuffix(tc.wantListPath, "/messages")+"/mailFolders/inbox/messages":
					if r.URL.Query().Get("$orderby") != "receivedDateTime desc" {
						t.Error("inbox recency is undefined")
					}
					_, _ = fmt.Fprint(w, `{"value":[{"id":"inbox-only"}]}`)
				case r.Method == http.MethodGet && r.URL.Path == tc.wantGetPath:
					_, _ = fmt.Fprint(w, `{"id":"message-id"}`)
				case r.Method == http.MethodPost && r.URL.Path == tc.wantSendMailPath:
					w.WriteHeader(http.StatusAccepted)
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			client := graph.NewClient(func(_ context.Context, scopes []string) (string, error) {
				if expectedScope != "" && !reflect.DeepEqual(scopes, []string{expectedScope}) {
					t.Errorf("wrong action scope: %v want %s", scopes, expectedScope)
				}
				return "token", nil
			})
			client.BaseURL = server.URL
			client.HTTPClient = server.Client()

			svc := New(client, tc.user)
			if _, _, err := svc.List(context.Background(), 10, "", "", ""); err != nil {
				t.Fatalf("List failed: %v", err)
			}
			items, _, err := svc.List(context.Background(), 10, "", "", "inbox")
			if err != nil || len(items) != 1 || items[0]["id"] != "inbox-only" {
				t.Fatalf("inbox selection leaked mailbox-wide messages: %v %v", items, err)
			}
			if _, err := svc.Get(context.Background(), "message-id"); err != nil {
				t.Fatalf("Get failed: %v", err)
			}
			if err := svc.Send(context.Background(), []string{"to@example.com"}, "subject", "body"); err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			ctx := context.Background()
			for _, action := range []struct {
				name, method, suffix, scope string
				body                        map[string]any
				run                         func() error
			}{
				{"reply", "POST", "/reply", "Mail.Send", map[string]any{"comment": "text"}, func() error { _, e := svc.Reply(ctx, "message-id", "text", false, false); return e }},
				{"reply all", "POST", "/replyAll", "Mail.Send", map[string]any{"comment": "text"}, func() error { _, e := svc.Reply(ctx, "message-id", "text", true, false); return e }},
				{"reply draft", "POST", "/createReply", "Mail.ReadWrite", map[string]any{"comment": "text"}, func() error {
					r, e := svc.Reply(ctx, "message-id", "text", false, true)
					if r["id"] != "result-id" {
						t.Error("draft result lost")
					}
					return e
				}},
				{"reply all draft", "POST", "/createReplyAll", "Mail.ReadWrite", map[string]any{"comment": "text"}, func() error { _, e := svc.Reply(ctx, "message-id", "text", true, true); return e }},
				{"forward", "POST", "/forward", "Mail.Send", map[string]any{"comment": "text", "toRecipients": []any{map[string]any{"emailAddress": map[string]any{"address": "to@example.com"}}}}, func() error { return svc.Forward(ctx, "message-id", "text", []string{"to@example.com"}) }},
				{"mark read", "PATCH", "", "Mail.ReadWrite", map[string]any{"isRead": true}, func() error { return svc.Mark(ctx, "message-id", true) }},
				{"mark unread", "PATCH", "", "Mail.ReadWrite", map[string]any{"isRead": false}, func() error { return svc.Mark(ctx, "message-id", false) }},
				{"move", "POST", "/move", "Mail.ReadWrite", map[string]any{"destinationId": "CustomFolder"}, func() error {
					r, e := svc.Move(ctx, "message-id", "CustomFolder")
					if r["id"] != "result-id" {
						t.Error("moved message result lost")
					}
					return e
				}},
				{"archive", "POST", "/move", "Mail.ReadWrite", map[string]any{"destinationId": "archive"}, func() error { _, e := svc.Move(ctx, "message-id", "archive"); return e }},
				{"restorable delete", "POST", "/move", "Mail.ReadWrite", map[string]any{"destinationId": "deleteditems"}, func() error { _, e := svc.Move(ctx, "message-id", "deleted"); return e }},
			} {
				expectedMethod, expectedPath, expectedScope, expectedBody = action.method, tc.wantGetPath+action.suffix, action.scope, action.body
				before := requests
				if err := action.run(); err != nil {
					t.Fatalf("%s: %v", action.name, err)
				}
				if requests != before+1 {
					t.Fatalf("%s made extra provider calls (a send must not require a read)", action.name)
				}
			}
			expectedBody = nil
			expectedScope = "Mail.Read"
			expectedMethod = "GET"
			mailbox := strings.TrimSuffix(tc.wantListPath, "/messages")
			expectedPath = mailbox + "/mailFolders"
			folders, _, err := svc.Folders(ctx, 20, "")
			if err != nil || len(folders) != 1 {
				t.Fatalf("folder discovery: %v %v", folders, err)
			}
			expectedPath = mailbox + "/mailFolders/deleteditems"
			deletedID, err := svc.DeletedItemsID(ctx)
			if err != nil || deletedID != "deleted-id" {
				t.Fatalf("Deleted Items identity lookup: %s %v", deletedID, err)
			}
			for _, folder := range []struct{ alias, id string }{{"sent", "sentitems"}, {"drafts", "drafts"}, {"archive", "archive"}, {"deleted", "deleteditems"}, {"junk", "junkemail"}, {"CustomFolder", "CustomFolder"}} {
				expectedPath = mailbox + "/mailFolders/" + folder.id + "/messages"
				if _, _, err := svc.List(ctx, 10, "", "", folder.alias); err != nil {
					t.Fatal(err)
				}
			}

		})
	}
}

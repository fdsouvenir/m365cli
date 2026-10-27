package mail

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/graph"
)

var listMailScopes = []string{"Mail.Read"}
var getMailScopes = []string{"Mail.Read"}
var sendMailScopes = []string{"Mail.Send"}
var draftMailScopes = []string{"Mail.ReadWrite"}

type Service struct {
	client      *graph.Client
	appOnlyUser string
}

func New(client *graph.Client, appOnlyUser string) *Service {
	return &Service{client: client, appOnlyUser: strings.TrimSpace(appOnlyUser)}
}

func (s *Service) List(ctx context.Context, max int, queryText string, page string, folder string) ([]map[string]any, string, error) {
	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}
	query.Set("$select", "id,subject,receivedDateTime,isRead,from")
	query.Set("$orderby", "receivedDateTime desc")

	headers := http.Header{}
	if strings.TrimSpace(queryText) != "" {
		query.Set("$search", fmt.Sprintf("\"%s\"", strings.TrimSpace(queryText)))
		headers.Set("ConsistencyLevel", "eventual")
		// Outlook search has its own server-side sent-time ordering.
		query.Del("$orderby")
	}

	endpoint := s.messagesEndpoint()
	if strings.TrimSpace(folder) != "" {
		endpoint = s.mailboxEndpoint() + "/mailFolders/" + url.PathEscape(FolderID(folder)) + "/messages"
	}
	if strings.TrimSpace(page) != "" {
		endpoint = strings.TrimSpace(page)
		query = nil
		if hasSearchQuery(page) {
			headers.Set("ConsistencyLevel", "eventual")
		}
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, query, nil, listMailScopes, headers)
	if err != nil {
		return nil, "", err
	}

	items, next, err := graph.DecodeODataPage(body)
	if err != nil {
		return nil, "", err
	}

	trimmed, trimmedNext := trimPage(items, next, max)
	return trimmed, trimmedNext, nil
}

func (s *Service) Get(ctx context.Context, id string) (map[string]any, error) {
	var payload map[string]any
	err := s.client.DoJSON(ctx, http.MethodGet, s.messagesEndpoint()+"/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, getMailScopes, &payload)
	return payload, err
}

func (s *Service) Send(ctx context.Context, to []string, subject string, body string) error {
	toRecipients := make([]map[string]any, 0, len(to))
	for _, address := range to {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		toRecipients = append(toRecipients, map[string]any{
			"emailAddress": map[string]any{"address": address},
		})
	}

	payload := map[string]any{
		"message": map[string]any{
			"subject": subject,
			"body": map[string]any{
				"contentType": "Text",
				"content":     body,
			},
			"toRecipients": toRecipients,
		},
		"saveToSentItems": true,
	}

	_, _, err := s.client.Do(ctx, http.MethodPost, s.sendMailEndpoint(), nil, payload, sendMailScopes, nil)
	return err
}

func (s *Service) Draft(ctx context.Context, to []string, subject string, body string) (map[string]any, error) {
	toRecipients := make([]map[string]any, 0, len(to))
	for _, address := range to {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		toRecipients = append(toRecipients, map[string]any{
			"emailAddress": map[string]any{"address": address},
		})
	}

	payload := map[string]any{
		"subject": subject,
		"body": map[string]any{
			"contentType": "Text",
			"content":     body,
		},
		"toRecipients": toRecipients,
	}

	var result map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, s.messagesEndpoint(), nil, payload, draftMailScopes, &result)
	return result, err
}

func (s *Service) messagesEndpoint() string {
	if s.appOnlyUser != "" {
		return "/users/" + url.PathEscape(s.appOnlyUser) + "/messages"
	}
	return "/me/messages"
}

func (s *Service) sendMailEndpoint() string {
	if s.appOnlyUser != "" {
		return "/users/" + url.PathEscape(s.appOnlyUser) + "/sendMail"
	}
	return "/me/sendMail"
}

func trimPage(items []map[string]any, next string, max int) ([]map[string]any, string) {
	if max <= 0 || len(items) <= max {
		return items, next
	}
	// The continuation skips the complete page, so retain every returned row.
	return items, next
}

func hasSearchQuery(page string) bool {
	u, err := url.Parse(strings.TrimSpace(page))
	if err != nil {
		return false
	}
	return strings.TrimSpace(u.Query().Get("$search")) != ""
}

// FolderID translates the CLI's friendly names; explicit IDs retain their case.
func FolderID(folder string) string {
	value := strings.TrimSpace(folder)
	switch strings.ToLower(value) {
	case "inbox":
		return "inbox"
	case "sent":
		return "sentitems"
	case "drafts":
		return "drafts"
	case "archive":
		return "archive"
	case "deleted":
		return "deleteditems"
	case "junk":
		return "junkemail"
	default:
		return value
	}
}

func (s *Service) mailboxEndpoint() string {
	if s.appOnlyUser != "" {
		return "/users/" + url.PathEscape(s.appOnlyUser)
	}
	return "/me"
}

func (s *Service) Folders(ctx context.Context, max int, page string) ([]map[string]any, string, error) {
	query := url.Values{"$select": {"id,displayName,parentFolderId,childFolderCount,totalItemCount,unreadItemCount"}}
	if max > 0 {
		query.Set("$top", fmt.Sprint(max))
	}
	endpoint := s.mailboxEndpoint() + "/mailFolders"
	if strings.TrimSpace(page) != "" {
		endpoint = strings.TrimSpace(page)
		query = nil
	}
	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, query, nil, listMailScopes, nil)
	if err != nil {
		return nil, "", err
	}
	return graph.DecodeODataPage(body)
}

// Reply lets Graph retain the thread and select the original reply recipients.
func (s *Service) Reply(ctx context.Context, id, body string, replyAll, draft bool) (map[string]any, error) {
	action := "reply"
	scopes := sendMailScopes
	if replyAll {
		action = "replyAll"
	}
	if draft {
		scopes = draftMailScopes
		action = "createReply"
		if replyAll {
			action = "createReplyAll"
		}
	}
	var result map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, s.messagesEndpoint()+"/"+url.PathEscape(strings.TrimSpace(id))+"/"+action, nil, map[string]any{"comment": body}, scopes, &result)
	return result, err
}

func (s *Service) Forward(ctx context.Context, id, body string, to []string) error {
	recipients := make([]map[string]any, 0, len(to))
	for _, address := range to {
		recipients = append(recipients, map[string]any{"emailAddress": map[string]any{"address": address}})
	}
	_, _, err := s.client.Do(ctx, http.MethodPost, s.messagesEndpoint()+"/"+url.PathEscape(strings.TrimSpace(id))+"/forward", nil, map[string]any{"comment": body, "toRecipients": recipients}, sendMailScopes, nil)
	return err
}

func (s *Service) Mark(ctx context.Context, id string, read bool) error {
	_, _, err := s.client.Do(ctx, http.MethodPatch, s.messagesEndpoint()+"/"+url.PathEscape(strings.TrimSpace(id)), nil, map[string]any{"isRead": read}, draftMailScopes, nil)
	return err
}

// Move also implements archive and restorable delete via Graph's move action.
// Never use DELETE here: a second delete must not remove mail from Deleted Items.
func (s *Service) Move(ctx context.Context, id, folder string) (map[string]any, error) {
	var result map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, s.messagesEndpoint()+"/"+url.PathEscape(strings.TrimSpace(id))+"/move", nil, map[string]any{"destinationId": FolderID(folder)}, draftMailScopes, &result)
	return result, err
}

// DeletedItemsID identifies the actual special folder without relying on its
// localized display name. Mail.ReadWrite also covers this Mail.Read request.
func (s *Service) DeletedItemsID(ctx context.Context) (string, error) {
	var folder struct {
		ID string `json:"id"`
	}
	err := s.client.DoJSON(ctx, http.MethodGet, s.mailboxEndpoint()+"/mailFolders/deleteditems", url.Values{"$select": {"id"}}, nil, listMailScopes, &folder)
	if err != nil {
		return "", err
	}
	if folder.ID == "" {
		return "", fmt.Errorf("Microsoft did not return the Deleted Items folder ID")
	}
	return folder.ID, nil
}

// Package excel operates on existing .xlsx workbooks in business OneDrive or
// SharePoint drives. Graph does not support personal or application-only Excel.
package excel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/graph"
)

var workbookScopes = []string{"Files.ReadWrite"}

type Service struct {
	client  *graph.Client
	base    string
	session string
}

func New(client *graph.Client, driveID, itemID, sessionID string) *Service {
	base := "/me/drive"
	if driveID != "" {
		base = "/drives/" + url.PathEscape(driveID)
	}
	return &Service{client: client, base: base + "/items/" + url.PathEscape(itemID) + "/workbook", session: sessionID}
}

func (s *Service) request(ctx context.Context, method, suffix string, body any) (map[string]any, error) {
	headers := http.Header{}
	if s.session != "" {
		headers.Set("workbook-session-id", s.session)
	}
	_, raw, err := s.client.Do(ctx, method, s.base+suffix, nil, body, workbookScopes, headers)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) Worksheets(ctx context.Context) ([]map[string]any, string, error) {
	// Paginate supplies the same permission requirement; collection calls do not
	// require a session (there are no uncommitted changes in persistent sessions).
	return s.client.Paginate(ctx, s.base+"/worksheets", nil, workbookScopes, 0)
}
func (s *Service) AddWorksheet(ctx context.Context, name string) (map[string]any, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("worksheet name is required")
	}
	return s.request(ctx, http.MethodPost, "/worksheets/add", map[string]any{"name": name})
}
func (s *Service) DeleteWorksheet(ctx context.Context, sheet string) (map[string]any, error) {
	return s.request(ctx, http.MethodDelete, "/worksheets/"+url.PathEscape(sheet), nil)
}
func rangeSuffix(sheet, address string) string {
	// Apostrophes in OData string literals are doubled before URL encoding.
	return "/worksheets/" + url.PathEscape(sheet) + "/" + url.PathEscape("range(address='"+strings.ReplaceAll(address, "'", "''")+"')")
}
func (s *Service) Range(ctx context.Context, sheet, address string) (map[string]any, error) {
	return s.request(ctx, http.MethodGet, rangeSuffix(sheet, address), nil)
}
func (s *Service) SetRange(ctx context.Context, sheet, address, field string, values [][]any) (map[string]any, error) {
	if field != "values" && field != "formulas" {
		return nil, fmt.Errorf("invalid range field")
	}
	if err := ValidateMatrix(values); err != nil {
		return nil, err
	}
	return s.request(ctx, http.MethodPatch, rangeSuffix(sheet, address), map[string]any{field: values})
}
func (s *Service) Tables(ctx context.Context) ([]map[string]any, string, error) {
	return s.client.Paginate(ctx, s.base+"/tables", nil, workbookScopes, 0)
}
func (s *Service) AddTable(ctx context.Context, address string, hasHeaders bool) (map[string]any, error) {
	return s.request(ctx, http.MethodPost, "/tables/add", map[string]any{"address": address, "hasHeaders": hasHeaders})
}
func (s *Service) AppendRows(ctx context.Context, table string, values [][]any) (map[string]any, error) {
	if err := ValidateMatrix(values); err != nil {
		return nil, err
	}
	return s.request(ctx, http.MethodPost, "/tables/"+url.PathEscape(table)+"/rows/add", map[string]any{"values": values})
}
func ValidateMatrix(values [][]any) error {
	if len(values) == 0 || len(values[0]) == 0 {
		return fmt.Errorf("expected a non-empty rectangular JSON array of rows")
	}
	width := len(values[0])
	for _, row := range values {
		if len(row) != width {
			return fmt.Errorf("all rows must have the same number of columns")
		}
		for _, v := range row {
			switch v.(type) {
			case nil, string, bool, float64:
			default:
				return fmt.Errorf("cells must be strings, numbers, booleans or null")
			}
		}
	}
	return nil
}

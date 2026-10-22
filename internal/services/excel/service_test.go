package excel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fdsouvenir/m365cli/internal/graph"
)

// Verify the caller-visible API contract, not internal helper structure: exact
// Graph routing, permission requirement and JSON cell types for workbook writes.
func TestWorkbookOperations(t *testing.T) {
	cases := []struct {
		name, method, path, body string
		run                      func(*Service) error
	}{
		{"range read", "GET", "/me/drive/items/book/workbook/worksheets/Sheet 1/range(address='A1:B2')", "", func(s *Service) error { _, e := s.Range(context.Background(), "Sheet 1", "A1:B2"); return e }},
		{"values update", "PATCH", "/me/drive/items/book/workbook/worksheets/Sheet1/range(address='A1:B1')", `{"values":[["hello",2]]}`, func(s *Service) error {
			_, e := s.SetRange(context.Background(), "Sheet1", "A1:B1", "values", [][]any{{"hello", float64(2)}})
			return e
		}},
		{"formula update", "PATCH", "/me/drive/items/book/workbook/worksheets/Sheet1/range(address='C1')", `{"formulas":[["=SUM(A1:B1)"]]}`, func(s *Service) error {
			_, e := s.SetRange(context.Background(), "Sheet1", "C1", "formulas", [][]any{{"=SUM(A1:B1)"}})
			return e
		}},
		{"worksheet add", "POST", "/me/drive/items/book/workbook/worksheets/add", `{"name":"Report"}`, func(s *Service) error { _, e := s.AddWorksheet(context.Background(), "Report"); return e }},
		{"table add", "POST", "/me/drive/items/book/workbook/tables/add", `{"address":"Sheet1!A1:B2","hasHeaders":true}`, func(s *Service) error { _, e := s.AddTable(context.Background(), "Sheet1!A1:B2", true); return e }},
		{"append rows", "POST", "/me/drive/items/book/workbook/tables/table-id/rows/add", `{"values":[[true,null]]}`, func(s *Service) error {
			_, e := s.AppendRows(context.Background(), "table-id", [][]any{{true, nil}})
			return e
		}},
		{"worksheet delete", "DELETE", "/me/drive/items/book/workbook/worksheets/sheet-id", "", func(s *Service) error { _, e := s.DeleteWorksheet(context.Background(), "sheet-id"); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("got %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer scoped-token" {
					t.Error("missing auth")
				}
				if tc.body != "" {
					var got, want any
					if e := json.NewDecoder(r.Body).Decode(&got); e != nil {
						t.Fatal(e)
					}
					if e := json.Unmarshal([]byte(tc.body), &want); e != nil {
						t.Fatal(e)
					}
					a, _ := json.Marshal(got)
					b, _ := json.Marshal(want)
					if string(a) != string(b) {
						t.Errorf("body %s, want %s", a, b)
					}
				}
				if tc.method == "DELETE" {
					w.WriteHeader(204)
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"id":"result"}`))
				}
			}))
			defer server.Close()
			client := graph.NewClient(func(_ context.Context, scopes []string) (string, error) {
				if strings.Join(scopes, " ") != "Files.ReadWrite" {
					t.Errorf("scopes %v", scopes)
				}
				return "scoped-token", nil
			})
			client.BaseURL = server.URL
			client.HTTPClient = server.Client()
			if err := tc.run(New(client, "", "book", "")); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("requests %d", calls)
			}
		})
	}
}

func TestDriveCollectionsFollowAllPages(t *testing.T) {
	calls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/drives/drive-id/items/book/workbook/worksheets" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") == "2" {
			w.Write([]byte(`{"value":[{"id":"2"}]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"value": []any{map[string]any{"id": "1"}}, "@odata.nextLink": server.URL + r.URL.Path + "?page=2"})
	}))
	defer server.Close()
	client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	items, next, err := New(client, "drive-id", "book", "").Worksheets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || next != "" || calls != 2 {
		t.Fatalf("items=%v next=%q calls=%d", items, next, calls)
	}
}

func TestMatrixRejectsMalformedWrites(t *testing.T) {
	for _, matrix := range [][][]any{nil, {{}}, {{1}, {1, 2}}, {{map[string]any{"nested": true}}}} {
		if err := ValidateMatrix(matrix); err == nil {
			t.Errorf("accepted matrix %v", matrix)
		}
	}
}

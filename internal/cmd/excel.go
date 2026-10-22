package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/outfmt"
	"github.com/fdsouvenir/m365cli/internal/services/excel"
)

type ExcelCmd struct {
	Worksheets ExcelWorksheetsCmd `cmd:"" help:"List, add or delete worksheets"`
	Range      ExcelRangeCmd      `cmd:"" help:"Read or update an addressed range"`
	Tables     ExcelTablesCmd     `cmd:"" help:"List or add workbook tables"`
	Rows       ExcelRowsCmd       `cmd:"" help:"Append rows to a table"`
}
type excelWorkbookFlags struct {
	Workbook string `name:"workbook" required:"" help:"Drive item ID of an existing business .xlsx workbook"`
	Drive    string `name:"drive" help:"Drive ID (defaults to signed-in user's OneDrive)"`
}

func (f *excelWorkbookFlags) service(ctx context.Context) (*excel.Service, error) {
	if strings.TrimSpace(f.Workbook) == "" {
		return nil, usage("--workbook must not be empty")
	}
	rt, err := resolveRuntime(ctx, capExcel)
	if err != nil {
		return nil, err
	}
	return excel.New(rt.Graph, f.Drive, f.Workbook, ""), nil
}

type ExcelWorksheetsCmd struct {
	List   ExcelWorksheetsListCmd   `cmd:"" help:"List worksheets"`
	Add    ExcelWorksheetsAddCmd    `cmd:"" help:"Add a named worksheet"`
	Delete ExcelWorksheetsDeleteCmd `cmd:"" help:"Delete a worksheet"`
}
type ExcelWorksheetsListCmd struct {
	excelWorkbookFlags `embed:""`
}

func (c *ExcelWorksheetsListCmd) Run(ctx context.Context) error {
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	items, next, err := s.Worksheets(ctx)
	if err != nil {
		return err
	}
	return excelCollection(ctx, "worksheets", items, next)
}

type ExcelWorksheetsAddCmd struct {
	excelWorkbookFlags `embed:""`
	Name               string `arg:"" required:"" help:"New worksheet name"`
	DryRun             bool   `help:"Preview without changing the workbook"`
}

func (c *ExcelWorksheetsAddCmd) Run(ctx context.Context) error {
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	if c.DryRun {
		return excelResult(ctx, map[string]any{"dry_run": true, "name": c.Name})
	}
	result, err := s.AddWorksheet(ctx, c.Name)
	if err != nil {
		return err
	}
	return excelResult(ctx, result)
}

type ExcelWorksheetsDeleteCmd struct {
	excelWorkbookFlags `embed:""`
	Sheet              string `arg:"" required:"" help:"Worksheet ID or name"`
	DryRun             bool   `help:"Preview without changing the workbook"`
}

func (c *ExcelWorksheetsDeleteCmd) Run(ctx context.Context, flags *RootFlags) error {
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	if c.DryRun {
		return excelResult(ctx, map[string]any{"dry_run": true, "delete": c.Sheet})
	}
	if err := confirmDestructive(ctx, flags, "delete worksheet "+c.Sheet); err != nil {
		return err
	}
	result, err := s.DeleteWorksheet(ctx, c.Sheet)
	if err != nil {
		return err
	}
	return excelResult(ctx, result)
}

type ExcelRangeCmd struct {
	Get ExcelRangeGetCmd `cmd:"" help:"Read values and formulas from a range"`
	Set ExcelRangeSetCmd `cmd:"" help:"Set values or formulas from a JSON matrix file"`
}
type excelRangeFlags struct {
	excelWorkbookFlags `embed:""`
	Sheet              string `name:"sheet" required:"" help:"Worksheet ID or name"`
	Address            string `name:"address" required:"" help:"A1 range, for example A1:C10"`
}
type ExcelRangeGetCmd struct {
	excelRangeFlags `embed:""`
}

func (c *ExcelRangeGetCmd) Run(ctx context.Context) error {
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	result, err := s.Range(ctx, c.Sheet, c.Address)
	if err != nil {
		return err
	}
	return excelResult(ctx, result)
}

type ExcelRangeSetCmd struct {
	excelRangeFlags `embed:""`
	File            string `name:"file" required:"" help:"JSON rectangular array of rows"`
	Field           string `name:"field" default:"values" enum:"values,formulas" help:"Cell property to update"`
	DryRun          bool   `help:"Preview without changing the workbook"`
}

func (c *ExcelRangeSetCmd) Run(ctx context.Context) error {
	values, err := readExcelMatrix(c.File)
	if err != nil {
		return err
	}
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	if c.DryRun {
		return excelResult(ctx, map[string]any{"dry_run": true, "sheet": c.Sheet, "address": c.Address, c.Field: values})
	}
	result, err := s.SetRange(ctx, c.Sheet, c.Address, c.Field, values)
	if err != nil {
		return err
	}
	return excelResult(ctx, result)
}

type ExcelTablesCmd struct {
	List ExcelTablesListCmd `cmd:"" help:"List workbook tables"`
	Add  ExcelTablesAddCmd  `cmd:"" help:"Create a table over an existing range"`
}
type ExcelTablesListCmd struct {
	excelWorkbookFlags `embed:""`
}

func (c *ExcelTablesListCmd) Run(ctx context.Context) error {
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	items, next, err := s.Tables(ctx)
	if err != nil {
		return err
	}
	return excelCollection(ctx, "tables", items, next)
}

type ExcelTablesAddCmd struct {
	excelWorkbookFlags `embed:""`
	Address            string `name:"address" required:"" help:"Sheet-qualified range, for example Sheet1!A1:C10"`
	HasHeaders         bool   `name:"has-headers" help:"First row contains column headers"`
	DryRun             bool   `help:"Preview without changing the workbook"`
}

func (c *ExcelTablesAddCmd) Run(ctx context.Context) error {
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	if c.DryRun {
		return excelResult(ctx, map[string]any{"dry_run": true, "address": c.Address, "hasHeaders": c.HasHeaders})
	}
	result, err := s.AddTable(ctx, c.Address, c.HasHeaders)
	if err != nil {
		return err
	}
	return excelResult(ctx, result)
}

type ExcelRowsCmd struct {
	Append ExcelRowsAppendCmd `cmd:"" help:"Append rows from a JSON matrix file"`
}
type ExcelRowsAppendCmd struct {
	excelWorkbookFlags `embed:""`
	Table              string `name:"table" required:"" help:"Table ID or name"`
	File               string `name:"file" required:"" help:"JSON rectangular array of rows"`
	DryRun             bool   `help:"Preview without changing the workbook"`
}

func (c *ExcelRowsAppendCmd) Run(ctx context.Context) error {
	values, err := readExcelMatrix(c.File)
	if err != nil {
		return err
	}
	s, err := c.service(ctx)
	if err != nil {
		return err
	}
	if c.DryRun {
		return excelResult(ctx, map[string]any{"dry_run": true, "table": c.Table, "values": values})
	}
	result, err := s.AppendRows(ctx, c.Table, values)
	if err != nil {
		return err
	}
	return excelResult(ctx, result)
}
func readExcelMatrix(path string) ([][]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values [][]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("read cell matrix: %w", err)
	}
	if err := excel.ValidateMatrix(values); err != nil {
		return nil, usage(err.Error())
	}
	return values, nil
}
func excelResult(ctx context.Context, result map[string]any) error {
	// Workbook matrices must remain intact; generic text map rendering loses them.
	return outfmt.WriteJSON(os.Stdout, result)
}
func excelCollection(ctx context.Context, name string, items []map[string]any, next string) error {
	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(os.Stdout, map[string]any{name: items, "next": next})
	}
	printItemTable(ctx, items, []string{"id", "name"})
	printNextPageHint(uiFromContext(ctx), next)
	return nil
}

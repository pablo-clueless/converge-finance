package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatXLSX ExportFormat = "xlsx"
	ExportFormatPDF  ExportFormat = "pdf"
	ExportFormatJSON ExportFormat = "json"
)

func (f ExportFormat) IsValid() bool {
	switch f {
	case ExportFormatCSV, ExportFormatXLSX, ExportFormatPDF, ExportFormatJSON:
		return true
	}
	return false
}

func (f ExportFormat) MimeType() string {
	switch f {
	case ExportFormatCSV:
		return "text/csv"
	case ExportFormatXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ExportFormatPDF:
		return "application/pdf"
	case ExportFormatJSON:
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func (f ExportFormat) FileExtension() string {
	switch f {
	case ExportFormatCSV:
		return ".csv"
	case ExportFormatXLSX:
		return ".xlsx"
	case ExportFormatPDF:
		return ".pdf"
	case ExportFormatJSON:
		return ".json"
	default:
		return ""
	}
}

type ExportTemplate struct {
	ID            common.ID
	EntityID      common.ID
	TemplateCode  string
	TemplateName  string
	Module        string
	ExportType    string
	Configuration map[string]any
	IsSystem      bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewExportTemplate(
	entityID common.ID,
	templateCode, templateName string,
	module, exportType string,
) *ExportTemplate {
	now := time.Now()
	return &ExportTemplate{
		ID:            common.NewID(),
		EntityID:      entityID,
		TemplateCode:  templateCode,
		TemplateName:  templateName,
		Module:        module,
		ExportType:    exportType,
		Configuration: make(map[string]any),
		IsSystem:      false,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (t *ExportTemplate) SetConfiguration(config map[string]any) {
	t.Configuration = config
	t.UpdatedAt = time.Now()
}

func (t *ExportTemplate) Activate() {
	t.IsActive = true
	t.UpdatedAt = time.Now()
}

func (t *ExportTemplate) Deactivate() {
	t.IsActive = false
	t.UpdatedAt = time.Now()
}

func (t *ExportTemplate) Update(
	templateName *string,
	configuration map[string]any,
) {
	if templateName != nil {
		t.TemplateName = *templateName
	}
	if configuration != nil {
		t.Configuration = configuration
	}
	t.UpdatedAt = time.Now()
}

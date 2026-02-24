package tool

import (
	"context"
	"errors"
	"strings"

	"genFu/internal/financial"
)

type CNInfoTool struct {
	client *financial.CNInfoClient
}

func NewCNInfoTool() *CNInfoTool {
	return &CNInfoTool{client: financial.NewCNInfoClient()}
}

func (t CNInfoTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "cninfo",
		Description: "fetch financial report announcements from cninfo.com.cn",
		Params: map[string]string{
			"action":    "string (query_announcements, download_pdf)",
			"symbol":    "string (stock code)",
			"page":      "number",
			"page_size": "number",
			"pdf_url":   "string (for download_pdf action)",
		},
		Required: []string{"action"},
	}
}

func (t CNInfoTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "cninfo", Error: err.Error()}, err
	}

	switch strings.ToLower(action) {
	case "query_announcements":
		symbol, err := requireString(args, "symbol")
		if err != nil {
			return ToolResult{Name: "cninfo", Error: err.Error()}, err
		}
		page, _ := optionalInt(args, "page")
		pageSize, _ := optionalInt(args, "page_size")
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 10
		}

		announcements, err := t.client.QueryAnnouncements(ctx, symbol, page, pageSize)
		if err != nil {
			return ToolResult{Name: "cninfo", Error: err.Error()}, err
		}
		return ToolResult{Name: "cninfo", Output: announcements}, nil

	case "download_pdf":
		pdfURL, err := requireString(args, "pdf_url")
		if err != nil {
			return ToolResult{Name: "cninfo", Error: err.Error()}, err
		}
		data, err := t.client.DownloadPDF(ctx, pdfURL)
		if err != nil {
			return ToolResult{Name: "cninfo", Error: err.Error()}, err
		}
		// 不返回PDF二进制数据，只返回大小
		return ToolResult{Name: "cninfo", Output: map[string]interface{}{
			"size":    len(data),
			"message": "PDF downloaded successfully",
		}}, nil

	default:
		return ToolResult{Name: "cninfo", Error: "unsupported_action"},
			errors.New("unsupported_action: " + action)
	}
}

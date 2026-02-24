package analyze

import "time"

type AnalyzeReport struct {
	ID         int64
	ReportType string
	Symbol     string
	Name       string
	Title      string
	Request    AnalyzeRequest
	Steps      []AnalyzeStep
	Summary    string
	CreatedAt  time.Time
}

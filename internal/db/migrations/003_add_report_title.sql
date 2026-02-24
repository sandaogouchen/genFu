-- Add title column for LLM-generated report titles
ALTER TABLE analyze_reports ADD COLUMN title TEXT NOT NULL DEFAULT '';

-- Create indexes for efficient filtering and searching
CREATE INDEX IF NOT EXISTS idx_analyze_reports_type_created_at
  ON analyze_reports (report_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_analyze_reports_title
  ON analyze_reports (title);

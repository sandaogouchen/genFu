// package main

// import (
// 	"database/sql"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"time"

// 	_ "github.com/mattn/go-sqlite3"
// )

// type ReportDetail struct {
// 	ID         int64     `json:"id"`
// 	ReportType string    `json:"report_type"`
// 	Symbol     string    `json:"symbol"`
// 	Name       string    `json:"name"`
// 	Title      string    `json:"title"`
// 	Request    string    `json:"request"`
// 	Steps      string    `json:"steps"`
// 	Summary    string    `json:"summary"`
// 	CreatedAt  time.Time `json:"created_at"`
// }

// func main() {
// 	db, err := sql.Open("sqlite3", "genfu.db")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer db.Close()

// 	// 检查所有报告的存储完整性
// 	fmt.Println("========================================")
// 	fmt.Println("报告存储完整性检查")
// 	fmt.Println("========================================")

// 	query := `
// 		SELECT id, report_type, symbol, name, title, request, steps, summary, created_at
// 		FROM analyze_reports
// 		ORDER BY created_at DESC
// 		LIMIT 20
// 	`

// 	rows, err := db.Query(query)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var report ReportDetail
// 		var createdAtStr string
// 		var title, request, steps, summary sql.NullString

// 		err := rows.Scan(
// 			&report.ID,
// 			&report.ReportType,
// 			&report.Symbol,
// 			&report.Name,
// 			&title,
// 			&request,
// 			&steps,
// 			&summary,
// 			&createdAtStr,
// 		)
// 		if err != nil {
// 			log.Fatal(err)
// 		}

// 		report.Title = title.String
// 		report.Request = request.String
// 		report.Steps = steps.String
// 		report.Summary = summary.String
// 		report.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)

// 		// 分析报告完整性
// 		fmt.Printf("\n【报告 #%d - %s】\n", report.ID, report.ReportType)
// 		fmt.Printf("  代码: %s | 名称: %s\n", report.Symbol, report.Name)
// 		fmt.Printf("  标题: %s\n", report.Title)

// 		// 检查标题
// 		if report.Title == "" {
// 			fmt.Println("  ⚠️  标题为空")
// 		} else {
// 			fmt.Println("  ✓ 标题已生成")
// 		}

// 		// 检查请求
// 		if report.Request == "" {
// 			fmt.Println("  ⚠️  请求为空")
// 		} else {
// 			fmt.Printf("  ✓ 请求长度: %d 字节\n", len(report.Request))
// 		}

// 		// 检查步骤
// 		if report.Steps == "" || report.Steps == "null" {
// 			fmt.Println("  ⚠️  步骤为空 (null)")
// 		} else if report.Steps == "[]" {
// 			fmt.Println("  ⚠️  步骤为空数组")
// 		} else {
// 			var steps []interface{}
// 			if err := json.Unmarshal([]byte(report.Steps), &steps); err == nil {
// 				fmt.Printf("  ✓ 包含 %d 个步骤\n", len(steps))
// 			} else {
// 				fmt.Printf("  ⚠️  步骤解析失败: %s\n", report.Steps[:50])
// 			}
// 		}

// 		// 检查总结
// 		if report.Summary == "" {
// 			fmt.Println("  ⚠️  总结为空")
// 		} else {
// 			fmt.Printf("  ✓ 总结长度: %d 字节\n", len(report.Summary))
// 		}

// 		fmt.Printf("  创建时间: %s\n", report.CreatedAt.Format("2006-01-02 15:04:05"))
// 	}

// 	// 统计各类型报告的完整性
// 	fmt.Println("\n========================================")
// 	fmt.Println("按类型统计")
// 	fmt.Println("========================================")

// 	typeQuery := `
// 		SELECT
// 			report_type,
// 			COUNT(*) as total,
// 			SUM(CASE WHEN title != '' THEN 1 ELSE 0 END) as has_title,
// 			SUM(CASE WHEN steps IS NOT NULL AND steps != 'null' AND steps != '[]' THEN 1 ELSE 0 END) as has_steps,
// 			SUM(CASE WHEN summary IS NOT NULL AND summary != '' THEN 1 ELSE 0 END) as has_summary
// 		FROM analyze_reports
// 		GROUP BY report_type
// 		ORDER BY total DESC
// 	`

// 	typeRows, err := db.Query(typeQuery)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer typeRows.Close()

// 	fmt.Printf("\n%-20s %6s %6s %6s %6s\n", "类型", "总数", "有标题", "有步骤", "有总结")
// 	fmt.Println(string(make([]byte, 56)))

// 	for typeRows.Next() {
// 		var reportType string
// 		var total, hasTitle, hasSteps, hasSummary int
// 		if err := typeRows.Scan(&reportType, &total, &hasTitle, &hasSteps, &hasSummary); err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("%-20s %6d %6d %6d %6d\n", reportType, total, hasTitle, hasSteps, hasSummary)
// 	}
// }

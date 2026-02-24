// package main

// import (
// 	"database/sql"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"time"

// 	_ "github.com/mattn/go-sqlite3"
// )

// type Report struct {
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
// 	// Open database
// 	db, err := sql.Open("sqlite3", "genfu.db")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer db.Close()

// 	// Query latest report
// 	var report Report
// 	var createdAtStr string
// 	query := `
// 		SELECT id, report_type, symbol, name, title, request, steps, summary, created_at
// 		FROM analyze_reports
// 		ORDER BY created_at DESC
// 		LIMIT 1
// 	`
// 	err = db.QueryRow(query).Scan(
// 		&report.ID,
// 		&report.ReportType,
// 		&report.Symbol,
// 		&report.Name,
// 		&report.Title,
// 		&report.Request,
// 		&report.Steps,
// 		&report.Summary,
// 		&createdAtStr,
// 	)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Parse created_at time
// 	report.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)

// 	// Print report details
// 	fmt.Println("=" + "===========================================")
// 	fmt.Printf("最新报告 (ID: %d)\n", report.ID)
// 	fmt.Println("=" + "===========================================")
// 	fmt.Printf("类型: %s\n", report.ReportType)
// 	fmt.Printf("代码: %s\n", report.Symbol)
// 	fmt.Printf("名称: %s\n", report.Name)
// 	fmt.Printf("标题: %s\n", report.Title)
// 	fmt.Printf("创建时间: %s\n", report.CreatedAt.Format("2006-01-02 15:04:05"))
// 	fmt.Println("=" + "===========================================")

// 	// Print request
// 	fmt.Println("\n【原始请求】")
// 	if report.Request != "" {
// 		var req interface{}
// 		if err := json.Unmarshal([]byte(report.Request), &req); err == nil {
// 			reqJSON, _ := json.MarshalIndent(req, "", "  ")
// 			fmt.Println(string(reqJSON))
// 		} else {
// 			fmt.Println(report.Request)
// 		}
// 	} else {
// 		fmt.Println("(空)")
// 	}

// 	// Print steps
// 	fmt.Println("\n【分析步骤】")
// 	if report.Steps != "" && report.Steps != "[]" {
// 		var steps []map[string]interface{}
// 		if err := json.Unmarshal([]byte(report.Steps), &steps); err == nil {
// 			for i, step := range steps {
// 				fmt.Printf("\n步骤 %d: %s\n", i+1, step["name"])
// 				if output, ok := step["output"].(string); ok && output != "" {
// 					// Truncate long output
// 					if len(output) > 200 {
// 						fmt.Printf("输出: %s...\n", output[:200])
// 					} else {
// 						fmt.Printf("输出: %s\n", output)
// 					}
// 				}
// 			}
// 		} else {
// 			fmt.Println("解析失败:", report.Steps)
// 		}
// 	} else {
// 		fmt.Println("(无步骤)")
// 	}

// 	// Print summary
// 	fmt.Println("\n【总结】")
// 	if report.Summary != "" {
// 		var summary interface{}
// 		if err := json.Unmarshal([]byte(report.Summary), &summary); err == nil {
// 			summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
// 			fmt.Println(string(summaryJSON))
// 		} else {
// 			// If not JSON, print as plain text
// 			if len(report.Summary) > 500 {
// 				fmt.Println(report.Summary[:500] + "...")
// 			} else {
// 				fmt.Println(report.Summary)
// 			}
// 		}
// 	} else {
// 		fmt.Println("(空)")
// 	}

// 	fmt.Println("\n" + "============================================")

// 	// Check for potential issues
// 	fmt.Println("\n【存储检查】")
// 	if report.Title == "" {
// 		fmt.Println("⚠️  警告: 标题为空")
// 	} else {
// 		fmt.Println("✓ 标题已生成:", report.Title)
// 	}

// 	if report.Steps == "" || report.Steps == "[]" {
// 		fmt.Println("⚠️  警告: 没有分析步骤")
// 	} else {
// 		var steps []interface{}
// 		json.Unmarshal([]byte(report.Steps), &steps)
// 		fmt.Printf("✓ 包含 %d 个分析步骤\n", len(steps))
// 	}

// 	if report.Summary == "" {
// 		fmt.Println("⚠️  警告: 总结为空")
// 	} else {
// 		fmt.Println("✓ 总结已存储 (长度:", len(report.Summary), "字符)")
// 	}

// 	// Query recent reports
// 	fmt.Println("\n【最近5条报告】")
// 	rows, err := db.Query(`
// 		SELECT id, report_type, symbol, name, title, created_at
// 		FROM analyze_reports
// 		ORDER BY created_at DESC
// 		LIMIT 5
// 	`)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var id int64
// 		var reportType, symbol, name, title, createdAt string
// 		if err := rows.Scan(&id, &reportType, &symbol, &name, &title, &createdAt); err != nil {
// 			log.Fatal(err)
// 		}
// 		fmt.Printf("ID:%d [%s] %s %s - %s (%s)\n",
// 			id, reportType, symbol, name, title, createdAt)
// 	}
// }

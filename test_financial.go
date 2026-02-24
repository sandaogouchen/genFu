//go:build ignore
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"genFu/internal/db"
	"genFu/internal/financial"
)

func main() {
	// 测试CNInfo客户端
	client := financial.NewCNInfoClient()
	ctx := context.Background()

	fmt.Println("=== 测试1: 查询公告 ===")
	announcements, err := client.QueryAnnouncements(ctx, "000592", 1, 3)
	if err != nil {
		log.Printf("查询公告失败: %v", err)
	} else {
		fmt.Printf("找到 %d 条公告\n", len(announcements))
		for i, ann := range announcements {
			fmt.Printf("[%d] %s - %s\n", i+1, ann.SecCode, ann.Title)
		}
	}

	// 测试数据库操作
	fmt.Println("\n=== 测试2: 数据库操作 ===")
	database, err := db.Open(db.Config{
		DSN:             "file::memory:?cache=shared",
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 应用迁移
	if err := db.ApplyMigrations(context.Background(), database); err != nil {
		log.Fatal(err)
	}

	repo := financial.NewRepository(database)

	// 测试保存报告
	fmt.Println("测试保存报告...")
	report := &financial.FinancialReport{
		Symbol:           "000592",
		AnnouncementID:   "test-123",
		Title:            "2025年年度报告",
		ReportType:       "年度报告",
		AnnouncementDate: time.Now(),
		PDFURL:           "http://example.com/test.pdf",
		Summary:          "这是一份测试摘要",
		KeyMetrics:       `{"revenue": "100亿元", "net_profit": "10亿元"}`,
	}
	if err := repo.SaveReport(ctx, report); err != nil {
		log.Printf("保存失败: %v", err)
	} else {
		fmt.Println("✓ 保存成功")
	}

	// 测试读取报告
	fmt.Println("测试读取报告...")
	cached, err := repo.GetCachedReport(ctx, "test-123")
	if err != nil {
		log.Printf("读取失败: %v", err)
	} else if cached == nil {
		fmt.Println("✗ 未找到报告")
	} else {
		fmt.Printf("✓ 读取成功: %s\n", cached.Title)
	}

	// 测试获取最新报告
	fmt.Println("测试获取最新报告...")
	latest, err := repo.GetLatestReports(ctx, "000592", 3)
	if err != nil {
		log.Printf("获取最新报告失败: %v", err)
	} else {
		fmt.Printf("✓ 找到 %d 条报告\n", len(latest))
	}

	fmt.Println("\n=== 测试3: 数据模型 ===")
	// 测试JSON序列化
	summary := financial.ReportSummary{
		Symbol:      "000592",
		CompanyName: "测试公司",
		ReportType:  "年度报告",
		Period:      "2025年",
		Summary:     "营收稳定增长",
		KeyMetrics: financial.Metrics{
			Revenue:     "100亿元",
			NetProfit:   "10亿元",
			GrossMargin: "40%",
			ROE:         "15%",
		},
		RiskFactors:   []string{"行业竞争加剧", "原材料成本上升"},
		GrowthDrivers: []string{"新品放量", "渠道拓展"},
		GeneratedAt:   time.Now(),
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Printf("JSON序列化失败: %v", err)
	} else {
		fmt.Println("✓ JSON序列化成功:")
		fmt.Println(string(data))
	}

	fmt.Println("\n=== 所有测试完成 ===")
}

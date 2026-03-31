package financial

import (
	"context"
	"fmt"
	"strings"
)

// ExchangeQuery 交易所原生 API 统一查询参数
type ExchangeQuery struct {
	Symbol    string
	Keyword   string
	StartDate string
	EndDate   string
	PageNum   int
	PageSize  int
}

// Defaults 填充默认值
func (q *ExchangeQuery) Defaults() {
	if q.PageNum <= 0 {
		q.PageNum = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 30
	}
}

// toExchangeQuery 从 AnnouncementQuery 转为 ExchangeQuery
func toExchangeQuery(q AnnouncementQuery) ExchangeQuery {
	return ExchangeQuery{
		Symbol:    q.Symbol,
		Keyword:   q.SearchKey,
		StartDate: q.StartDate,
		EndDate:   q.EndDate,
		PageNum:   q.PageNum,
		PageSize:  q.PageSize,
	}
}

// isSSEStock 根据股票代码前缀判断是否为上交所股票
// 6 开头 = 上交所主板，688 开头 = 科创板（上交所）
func isSSEStock(symbol string) bool {
	if len(symbol) < 1 {
		return false
	}
	// 处理逗号分隔的多股票——取第一个判断
	first := strings.Split(symbol, ",")[0]
	first = strings.TrimSpace(first)
	return strings.HasPrefix(first, "6")
}

// isSZSEStock 根据股票代码前缀判断是否为深交所股票
// 0 开头 = 深交所主板/中小板，3 开头 = 创业板
func isSZSEStock(symbol string) bool {
	if len(symbol) < 1 {
		return false
	}
	first := strings.Split(symbol, ",")[0]
	first = strings.TrimSpace(first)
	return strings.HasPrefix(first, "0") || strings.HasPrefix(first, "3")
}

// isBSEStock 根据股票代码前缀判断是否为北交所股票
// 8 开头 = 北交所
func isBSEStock(symbol string) bool {
	if len(symbol) < 1 {
		return false
	}
	first := strings.Split(symbol, ",")[0]
	first = strings.TrimSpace(first)
	return strings.HasPrefix(first, "8") || strings.HasPrefix(first, "4")
}

// SearchWithFallback CNInfo 为主查询 + 交易所降级
// 当 CNInfo 查询失败或返回空结果时，自动降级到交易所原生 API
func (s *Service) SearchWithFallback(ctx context.Context, query AnnouncementQuery) (*AnnouncementResult, error) {
	query.Defaults()

	// 首先尝试 CNInfo
	result, err := s.client.QueryAnnouncements(ctx, query)
	if err == nil && len(result.Announcements) > 0 {
		return result, nil
	}

	// CNInfo 失败或无结果——尝试交易所降级
	if s.sse == nil && s.szse == nil {
		if err != nil {
			return nil, fmt.Errorf("cninfo query failed and no exchange fallback available: %w", err)
		}
		return result, nil // 返回空结果
	}

	exchangeQuery := toExchangeQuery(query)

	// 根据股票代码判断交易所
	if query.Symbol != "" {
		if isSSEStock(query.Symbol) && s.sse != nil {
			fallback, fErr := s.sse.QueryBulletins(ctx, exchangeQuery)
			if fErr == nil {
				return fallback, nil
			}
		}
		if isSZSEStock(query.Symbol) && s.szse != nil {
			fallback, fErr := s.szse.QueryAnnouncements(ctx, exchangeQuery)
			if fErr == nil {
				return fallback, nil
			}
		}
	}

	// 无法确定交易所或降级也失败，返回原始结果或错误
	if err != nil {
		return nil, err
	}
	return result, nil
}
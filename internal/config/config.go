package config

import (
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Server     ServerConfig     `yaml:"server"`
	PG         PGConfig         `yaml:"pg"`
	LLM        LLMConfig        `yaml:"llm"`
	Embedding  EmbeddingConfig  `yaml:"embedding"`
	EastMoney  EastMoneyConfig  `yaml:"eastmoney"`
	Tushare    TushareConfig    `yaml:"tushare"`
	RSSHub     RSSHubConfig     `yaml:"rsshub"`
	News       NewsConfig       `yaml:"news"`
	Decision   DecisionConfig   `yaml:"decision"`
	NextOpen   NextOpenConfig   `yaml:"next_open"`
	Access     AccessConfig     `yaml:"access"`
	RuleEngine RuleEngineConfig `yaml:"rule_engine,omitempty"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type PGConfig struct {
	DSN             string `yaml:"dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

type LLMConfig struct {
	Endpoint    string  `yaml:"endpoint"`
	APIKey      string  `yaml:"api_key"`
	Model       string  `yaml:"model"`
	Timeout     string  `yaml:"timeout"`
	Temperature float64 `yaml:"temperature"`
	RetryCount  int     `yaml:"retry_count"`
	RetryDelay  string  `yaml:"retry_delay"`
	HedgeDelay  string  `yaml:"hedge_delay"`
	MaxInflight int     `yaml:"max_inflight"`
}

type EmbeddingConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	Timeout  string `yaml:"timeout"`
}

type EastMoneyConfig struct {
	Timeout     string `yaml:"timeout"`
	MaxRetries  int    `yaml:"max_retries"`
	MinInterval string `yaml:"min_interval"`
	Referer     string `yaml:"referer"`
	UserAgent   string `yaml:"user_agent"`
}

type TushareConfig struct {
	Token      string `yaml:"token"`
	BaseURL    string `yaml:"base_url"`
	Timeout    string `yaml:"timeout"`
	MaxRetries int    `yaml:"max_retries"`
	RateLimit  int    `yaml:"rate_limit"`
}

type RSSHubConfig struct {
	BaseURL      string   `yaml:"base_url"`
	Routes       []string `yaml:"routes"`
	PollInterval string   `yaml:"poll_interval"`
	MaxItems     int      `yaml:"max_items"`
	Timeout      string   `yaml:"timeout"`
}

type NewsConfig struct {
	AccountID       int64          `yaml:"account_id"`
	BriefLimit      int            `yaml:"brief_limit"`
	Keywords        []string       `yaml:"keywords"`
	PipelineEnabled bool           `yaml:"pipeline_enabled"`
	Pipeline        PipelineConfig `yaml:"pipeline"`
}

type DecisionConfig struct {
	MaxSingleOrderRatio    float64 `yaml:"max_single_order_ratio"`
	MaxSymbolExposureRatio float64 `yaml:"max_symbol_exposure_ratio"`
	MaxDailyTradeRatio     float64 `yaml:"max_daily_trade_ratio"`
	MinConfidence          float64 `yaml:"min_confidence"`
}

type PipelineConfig struct {
	PreMarketTime            string  `yaml:"pre_market_time"`
	IntradayInterval         string  `yaml:"intraday_interval"`
	TradingStart             string  `yaml:"trading_start"`
	TradingEnd               string  `yaml:"trading_end"`
	LookbackDuration         string  `yaml:"lookback_duration"`
	EventImpactEnabled       *bool   `yaml:"event_impact_enabled"`
	CausalVerifierEnabled    *bool   `yaml:"causal_verifier_enabled"`
	EventImpactBatchSize     int     `yaml:"event_impact_batch_size"`
	VerifierMaxAnalyze       int     `yaml:"verifier_max_analyze"`
	VerifierWeakThreshold    float64 `yaml:"verifier_weak_threshold"`
	VerifierInvalidThreshold float64 `yaml:"verifier_invalid_threshold"`
}

type NextOpenConfig struct {
	Enabled   bool  `yaml:"enabled"`
	Hour      int   `yaml:"hour"`
	Minute    int   `yaml:"minute"`
	AccountID int64 `yaml:"account_id"`
	NewsLimit int   `yaml:"news_limit"`
}

type AccessConfig struct {
	Enabled    bool     `yaml:"enabled"`
	APIKeys    []string `yaml:"api_keys"`
	AllowPaths []string `yaml:"allow_paths"`
}

// RuleEngineConfig holds configuration for the dynamic SL/TP rule engine.
type RuleEngineConfig struct {
	Enabled       bool   `yaml:"enabled"`
	PollInterval  string `yaml:"poll_interval"`
	DefaultCooldown string `yaml:"default_cooldown"`
	TradingHours  struct {
		Start    string `yaml:"start"`
		End      string `yaml:"end"`
		Timezone string `yaml:"timezone"`
	} `yaml:"trading_hours"`
}

type NormalizedConfig struct {
	Server    ServerConfig
	PG        NormalizedPGConfig
	LLM       NormalizedLLMConfig
	Embedding NormalizedEmbeddingConfig
	EastMoney NormalizedEastMoneyConfig
	Tushare   struct {
		Token      string
		BaseURL    string
		Timeout    time.Duration
		MaxRetries int
		RateLimit  int
	}
	RSSHub   NormalizedRSSHubConfig
	News     NormalizedNewsConfig
	Decision NormalizedDecisionConfig
	NextOpen NormalizedNextOpenConfig
	Access   NormalizedAccessConfig
}

type NormalizedPGConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type NormalizedLLMConfig struct {
	Endpoint    string
	APIKey      string
	Model       string
	Timeout     time.Duration
	Temperature float64
	RetryCount  int
	RetryDelay  time.Duration
	HedgeDelay  time.Duration
	MaxInflight int
}

type NormalizedEmbeddingConfig struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
	Timeout  time.Duration
}

type NormalizedEastMoneyConfig struct {
	Timeout     time.Duration
	MaxRetries  int
	MinInterval time.Duration
	Referer     string
	UserAgent   string
}

type NormalizedRSSHubConfig struct {
	BaseURL      string
	Routes       []string
	PollInterval time.Duration
	MaxItems     int
	Timeout      time.Duration
}

type NormalizedNewsConfig struct {
	AccountID       int64
	BriefLimit      int
	Keywords        []string
	PipelineEnabled bool
	Pipeline        NormalizedPipelineConfig
}

type NormalizedDecisionConfig struct {
	MaxSingleOrderRatio    float64
	MaxSymbolExposureRatio float64
	MaxDailyTradeRatio     float64
	MinConfidence          float64
}

type NormalizedPipelineConfig struct {
	PreMarketTime            string
	IntradayInterval         time.Duration
	TradingStart             string
	TradingEnd               string
	LookbackDuration         time.Duration
	EventImpactEnabled       bool
	CausalVerifierEnabled    bool
	EventImpactBatchSize     int
	VerifierMaxAnalyze       int
	VerifierWeakThreshold    float64
	VerifierInvalidThreshold float64
}

type NormalizedAccessConfig struct {
	Enabled    bool
	APIKeys    []string
	AllowPaths []string
}

type NormalizedNextOpenConfig struct {
	Enabled   bool
	Hour      int
	Minute    int
	AccountID int64
	NewsLimit int
}

func Load(path string) (NormalizedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return NormalizedConfig{}, err
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return NormalizedConfig{}, err
	}
	return normalize(cfg)
}

func normalize(cfg AppConfig) (NormalizedConfig, error) {
	result := NormalizedConfig{
		Server: cfg.Server,
		PG: NormalizedPGConfig{
			DSN:          cfg.PG.DSN,
			MaxOpenConns: cfg.PG.MaxOpenConns,
			MaxIdleConns: cfg.PG.MaxIdleConns,
		},
		LLM: NormalizedLLMConfig{
			Endpoint:    cfg.LLM.Endpoint,
			APIKey:      cfg.LLM.APIKey,
			Model:       cfg.LLM.Model,
			Temperature: cfg.LLM.Temperature,
			RetryCount:  cfg.LLM.RetryCount,
			MaxInflight: cfg.LLM.MaxInflight,
		},
		Embedding: NormalizedEmbeddingConfig{
			Provider: cfg.Embedding.Provider,
			APIKey:   cfg.Embedding.APIKey,
			Model:    cfg.Embedding.Model,
			BaseURL:  cfg.Embedding.BaseURL,
		},
		EastMoney: NormalizedEastMoneyConfig{},
		RSSHub: NormalizedRSSHubConfig{
			BaseURL:  cfg.RSSHub.BaseURL,
			Routes:   cfg.RSSHub.Routes,
			MaxItems: cfg.RSSHub.MaxItems,
		},
		News: NormalizedNewsConfig{
			AccountID:       cfg.News.AccountID,
			BriefLimit:      cfg.News.BriefLimit,
			Keywords:        cfg.News.Keywords,
			PipelineEnabled: cfg.News.PipelineEnabled,
			Pipeline: NormalizedPipelineConfig{
				PreMarketTime:            cfg.News.Pipeline.PreMarketTime,
				IntradayInterval:         30 * time.Minute,
				TradingStart:             cfg.News.Pipeline.TradingStart,
				TradingEnd:               cfg.News.Pipeline.TradingEnd,
				LookbackDuration:         24 * time.Hour,
				EventImpactEnabled:       true,
				CausalVerifierEnabled:    true,
				EventImpactBatchSize:     cfg.News.Pipeline.EventImpactBatchSize,
				VerifierMaxAnalyze:       cfg.News.Pipeline.VerifierMaxAnalyze,
				VerifierWeakThreshold:    cfg.News.Pipeline.VerifierWeakThreshold,
				VerifierInvalidThreshold: cfg.News.Pipeline.VerifierInvalidThreshold,
			},
		},
		Decision: NormalizedDecisionConfig{
			MaxSingleOrderRatio:    cfg.Decision.MaxSingleOrderRatio,
			MaxSymbolExposureRatio: cfg.Decision.MaxSymbolExposureRatio,
			MaxDailyTradeRatio:     cfg.Decision.MaxDailyTradeRatio,
			MinConfidence:          cfg.Decision.MinConfidence,
		},
		NextOpen: NormalizedNextOpenConfig{
			Enabled:   cfg.NextOpen.Enabled,
			Hour:      cfg.NextOpen.Hour,
			Minute:    cfg.NextOpen.Minute,
			AccountID: cfg.NextOpen.AccountID,
			NewsLimit: cfg.NextOpen.NewsLimit,
		},
		Access: NormalizedAccessConfig{
			Enabled:    cfg.Access.Enabled,
			APIKeys:    cfg.Access.APIKeys,
			AllowPaths: cfg.Access.AllowPaths,
		},
	}
	if result.Server.Port == 0 {
		result.Server.Port = 8080
	}
	if result.PG.MaxOpenConns == 0 {
		result.PG.MaxOpenConns = 10
	}
	if result.PG.MaxIdleConns == 0 {
		result.PG.MaxIdleConns = 10
	}
	lifetime := cfg.PG.ConnMaxLifetime
	if lifetime == "" {
		lifetime = "30m"
	}
	d, err := time.ParseDuration(lifetime)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_conn_max_lifetime")
	}
	result.PG.ConnMaxLifetime = d
	if result.PG.DSN == "" {
		return NormalizedConfig{}, errors.New("missing_sqlite_dsn")
	}
	retryDelay := cfg.LLM.RetryDelay
	if retryDelay == "" {
		retryDelay = "3m"
	}
	parsedRetryDelay, err := time.ParseDuration(retryDelay)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_llm_retry_delay")
	}
	result.LLM.RetryDelay = parsedRetryDelay

	// 解析LLM超时配置
	llmTimeout := cfg.LLM.Timeout
	if llmTimeout == "" {
		llmTimeout = "90s"
	}
	parsedLLMTimeout, err := time.ParseDuration(llmTimeout)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_llm_timeout")
	}
	result.LLM.Timeout = parsedLLMTimeout
	log.Printf("LLM超时配置: %v", parsedLLMTimeout)

	hedgeDelay := cfg.LLM.HedgeDelay
	if hedgeDelay == "" {
		hedgeDelay = "0ms"
	}
	parsedHedgeDelay, err := time.ParseDuration(hedgeDelay)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_llm_hedge_delay")
	}
	result.LLM.HedgeDelay = parsedHedgeDelay

	if result.LLM.RetryCount < 0 {
		result.LLM.RetryCount = 0
	}
	if result.LLM.MaxInflight <= 0 {
		result.LLM.MaxInflight = 4
	}

	// Embedding defaults
	if result.Embedding.Provider == "" {
		result.Embedding.Provider = "openai"
	}
	if result.Embedding.Model == "" {
		result.Embedding.Model = "text-embedding-3-small"
	}
	embedTimeout := cfg.Embedding.Timeout
	if embedTimeout == "" {
		embedTimeout = "30s"
	}
	parsedEmbedTimeout, err := time.ParseDuration(embedTimeout)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_embedding_timeout")
	}
	result.Embedding.Timeout = parsedEmbedTimeout

	eastmoneyTimeout := strings.TrimSpace(cfg.EastMoney.Timeout)
	if eastmoneyTimeout == "" {
		eastmoneyTimeout = "15s"
	}
	parsedEastMoneyTimeout, err := time.ParseDuration(eastmoneyTimeout)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_eastmoney_timeout")
	}
	result.EastMoney.Timeout = parsedEastMoneyTimeout

	result.EastMoney.MaxRetries = cfg.EastMoney.MaxRetries
	if result.EastMoney.MaxRetries <= 0 {
		result.EastMoney.MaxRetries = 3
	}

	eastmoneyMinInterval := strings.TrimSpace(cfg.EastMoney.MinInterval)
	if eastmoneyMinInterval == "" {
		eastmoneyMinInterval = "200ms"
	}
	parsedEastMoneyMinInterval, err := time.ParseDuration(eastmoneyMinInterval)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_eastmoney_min_interval")
	}
	result.EastMoney.MinInterval = parsedEastMoneyMinInterval

	result.EastMoney.Referer = strings.TrimSpace(cfg.EastMoney.Referer)
	if result.EastMoney.Referer == "" {
		result.EastMoney.Referer = "https://quote.eastmoney.com/center/gridlist.html"
	}

	result.EastMoney.UserAgent = strings.TrimSpace(cfg.EastMoney.UserAgent)
	if result.EastMoney.UserAgent == "" {
		result.EastMoney.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	}

	// Tushare
	result.Tushare.Token = cfg.Tushare.Token
	result.Tushare.BaseURL = cfg.Tushare.BaseURL
	if result.Tushare.BaseURL == "" {
		result.Tushare.BaseURL = "http://api.tushare.pro"
	}
	if cfg.Tushare.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Tushare.Timeout); err == nil {
			result.Tushare.Timeout = d
		}
	}
	if result.Tushare.Timeout == 0 {
		result.Tushare.Timeout = 15 * time.Second
	}
	result.Tushare.MaxRetries = cfg.Tushare.MaxRetries
	if result.Tushare.MaxRetries <= 0 {
		result.Tushare.MaxRetries = 3
	}
	result.Tushare.RateLimit = cfg.Tushare.RateLimit
	if result.Tushare.RateLimit <= 0 || result.Tushare.RateLimit > 500 {
		result.Tushare.RateLimit = 200
	}
	if result.Tushare.Token != "" {
		log.Printf("Tushare Pro 已配置 (token: %s***)", result.Tushare.Token[:min(6, len(result.Tushare.Token))])
	} else {
		log.Printf("Tushare Pro 未配置 (token 为空，Tushare Tool 将不注册)")
	}

	if result.RSSHub.BaseURL == "" {
		result.RSSHub.BaseURL = "https://rsshub.app"
	}
	if result.RSSHub.MaxItems <= 0 {
		result.RSSHub.MaxItems = 20
	}
	rssPoll := cfg.RSSHub.PollInterval
	if rssPoll == "" {
		rssPoll = "10m"
	}
	parsedPoll, err := time.ParseDuration(rssPoll)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_rsshub_poll_interval")
	}
	result.RSSHub.PollInterval = parsedPoll

	// 解析RSSHub超时配置
	rssTimeout := cfg.RSSHub.Timeout
	if rssTimeout == "" {
		rssTimeout = "10s"
	}
	parsedRSSTimeout, err := time.ParseDuration(rssTimeout)
	if err != nil {
		return NormalizedConfig{}, errors.New("invalid_rsshub_timeout")
	}
	result.RSSHub.Timeout = parsedRSSTimeout

	if result.News.BriefLimit <= 0 {
		result.News.BriefLimit = 20
	}
	if result.News.AccountID == 0 {
		result.News.AccountID = 1
	}
	if result.Decision.MaxSingleOrderRatio <= 0 || result.Decision.MaxSingleOrderRatio > 1 {
		result.Decision.MaxSingleOrderRatio = 0.20
	}
	if result.Decision.MaxSymbolExposureRatio <= 0 || result.Decision.MaxSymbolExposureRatio > 1 {
		result.Decision.MaxSymbolExposureRatio = 0.30
	}
	if result.Decision.MaxDailyTradeRatio <= 0 || result.Decision.MaxDailyTradeRatio > 1 {
		result.Decision.MaxDailyTradeRatio = 0.40
	}
	if result.Decision.MinConfidence <= 0 || result.Decision.MinConfidence > 1 {
		result.Decision.MinConfidence = 0.55
	}

	// Pipeline defaults
	if result.News.Pipeline.PreMarketTime == "" {
		result.News.Pipeline.PreMarketTime = "08:30"
	}
	if cfg.News.Pipeline.IntradayInterval != "" {
		parsedInterval, err := time.ParseDuration(cfg.News.Pipeline.IntradayInterval)
		if err != nil {
			return NormalizedConfig{}, errors.New("invalid_intraday_interval")
		}
		result.News.Pipeline.IntradayInterval = parsedInterval
	}
	if result.News.Pipeline.TradingStart == "" {
		result.News.Pipeline.TradingStart = "09:30"
	}
	if result.News.Pipeline.TradingEnd == "" {
		result.News.Pipeline.TradingEnd = "15:00"
	}
	if cfg.News.Pipeline.LookbackDuration != "" {
		parsedLookback, err := time.ParseDuration(cfg.News.Pipeline.LookbackDuration)
		if err != nil {
			return NormalizedConfig{}, errors.New("invalid_lookback_duration")
		}
		result.News.Pipeline.LookbackDuration = parsedLookback
	}
	if cfg.News.Pipeline.EventImpactEnabled != nil {
		result.News.Pipeline.EventImpactEnabled = *cfg.News.Pipeline.EventImpactEnabled
	}
	if cfg.News.Pipeline.CausalVerifierEnabled != nil {
		result.News.Pipeline.CausalVerifierEnabled = *cfg.News.Pipeline.CausalVerifierEnabled
	}
	if result.News.Pipeline.EventImpactBatchSize <= 0 {
		result.News.Pipeline.EventImpactBatchSize = 10
	}
	if result.News.Pipeline.VerifierMaxAnalyze <= 0 {
		result.News.Pipeline.VerifierMaxAnalyze = 5
	}
	if result.News.Pipeline.VerifierWeakThreshold <= 0 || result.News.Pipeline.VerifierWeakThreshold >= 1 {
		result.News.Pipeline.VerifierWeakThreshold = 0.6
	}
	if result.News.Pipeline.VerifierInvalidThreshold <= 0 || result.News.Pipeline.VerifierInvalidThreshold >= result.News.Pipeline.VerifierWeakThreshold {
		result.News.Pipeline.VerifierInvalidThreshold = 0.4
	}

	if result.NextOpen.Hour == 0 && result.NextOpen.Minute == 0 {
		result.NextOpen.Hour = 19
		result.NextOpen.Minute = 30
	}
	if result.NextOpen.AccountID == 0 {
		result.NextOpen.AccountID = result.News.AccountID
	}
	if result.NextOpen.AccountID == 0 {
		result.NextOpen.AccountID = 1
	}
	if result.NextOpen.NewsLimit <= 0 {
		result.NextOpen.NewsLimit = 10
	}
	if len(result.Access.AllowPaths) == 0 {
		result.Access.AllowPaths = []string{"/healthz", "/docs", "/openapi.json"}
	}
	if result.Access.Enabled && len(result.Access.APIKeys) == 0 {
		return NormalizedConfig{}, errors.New("missing_access_api_keys")
	}
	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

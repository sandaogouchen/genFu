package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"genFu/internal/analyze"
	"genFu/internal/api"
	"genFu/internal/chat"
	"genFu/internal/decision"
	"genFu/internal/news"
	"genFu/internal/router"
	stockpicker "genFu/internal/stockpicker"
	"genFu/internal/tool"
	"genFu/internal/workflow"
	"genFu/internal/ws"
)

type Server struct {
	router           *router.Router
	registry         *tool.Registry
	analyzer         *analyze.Analyzer
	decision         *decision.Service
	stockpicker      *stockpicker.Service
	stockpickerGuide *stockpicker.GuideRepository
	chat             *chat.Service
	stockWF          *workflow.StockWorkflow
	ocr              http.Handler
	newsPipeline     *news.Pipeline
	newsRepo         *news.Repository
}

func NewServer(r *router.Router, registry *tool.Registry, analyzer *analyze.Analyzer, decisionSvc *decision.Service, stockpickerSvc *stockpicker.Service, stockpickerGuide *stockpicker.GuideRepository, chatSvc *chat.Service, stockWF *workflow.StockWorkflow, ocr http.Handler, newsPipeline *news.Pipeline, newsRepo *news.Repository) *Server {
	return &Server{router: r, registry: registry, analyzer: analyzer, decision: decisionSvc, stockpicker: stockpickerSvc, stockpickerGuide: stockpickerGuide, chat: chatSvc, stockWF: stockWF, ocr: ocr, newsPipeline: newsPipeline, newsRepo: newsRepo}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	if s.chat != nil {
		mux.Handle("/ws/generate", chat.NewWSHandler(s.chat))
	} else {
		mux.Handle("/ws/generate", ws.NewHandler(s.router, s.registry))
	}
	if s.analyzer != nil {
		mux.Handle("/api/analyze", analyze.NewHandler(s.analyzer))
		mux.Handle("/sse/analyze", analyze.NewSSEHandler(s.analyzer))
		if repo := s.analyzer.GetRepo(); repo != nil {
			mux.Handle("/api/reports", analyze.NewListHandler(repo))
			mux.Handle("/api/reports/", analyze.NewDetailHandler(repo))
		}
	}
	if s.registry != nil {
		mux.Handle("/api/investment", api.NewInvestmentHandler(s.registry))
		mux.Handle("/api/marketdata", api.NewMarketDataHandler(s.registry))
	}
	if s.ocr != nil {
		mux.Handle("/api/investment/ocr_holdings", s.ocr)
	}
	if s.decision != nil {
		mux.Handle("/api/decision", decision.NewHandler(s.decision))
		mux.Handle("/sse/decision", decision.NewSSEHandler(s.decision))
	}
	if s.stockpicker != nil {
		stockpickerHandler := stockpicker.NewHandler(s.stockpicker, s.stockpickerGuide)
		mux.Handle("/api/stockpicker", stockpickerHandler)
		// Operation guide APIs
		mux.HandleFunc("/api/operation-guide", stockpickerHandler.GetOperationGuide)
		mux.HandleFunc("/api/operation-guide/", func(w http.ResponseWriter, r *http.Request) {
			// Route to GetOperationGuideByID if path has ID
			if strings.HasPrefix(r.URL.Path, "/api/operation-guide/") && len(strings.TrimPrefix(r.URL.Path, "/api/operation-guide/")) > 0 {
				stockpickerHandler.GetOperationGuideByID(w, r)
			} else {
				http.NotFound(w, r)
			}
		})
	}
	if s.chat != nil {
		chatHandler := chat.NewHandler(s.chat)
		sseChatHandler := chat.NewSSEHandler(s.chat)
		// Inject analyze repository if available
		if s.analyzer != nil {
			if repo := s.analyzer.GetRepo(); repo != nil {
				sseChatHandler.SetAnalyzeRepo(repo)
			}
		}
		mux.Handle("/api/chat", chatHandler)
		mux.Handle("/sse/chat", sseChatHandler)
		mux.Handle("/ws/chat", chat.NewWSHandler(s.chat))
		mux.Handle("/api/chat/history", chat.NewHistoryHandler(s.chat))
	}
	if s.stockWF != nil {
		stockHandler := workflow.NewStockHandler(s.stockWF)
		stockSSEHandler := workflow.NewStockSSEHandler(s.stockWF)
		// Inject analyze repository if available
		if s.analyzer != nil {
			if repo := s.analyzer.GetRepo(); repo != nil {
				stockSSEHandler.SetAnalyzeRepo(repo)
			}
		}
		mux.Handle("/api/workflow/stock", stockHandler)
		mux.Handle("/sse/workflow/stock", stockSSEHandler)
	}
	// News events API
	if s.newsPipeline != nil {
		mux.Handle("/api/news/events", news.NewListEventsHandler(s.newsRepo, s.newsPipeline))
		mux.Handle("/api/news/events/", news.NewGetEventHandler(s.newsRepo, s.newsPipeline))
		mux.Handle("/api/news/briefing", news.NewGetBriefingHandler(s.newsRepo, s.newsPipeline))
		mux.Handle("/api/news/analyze", news.NewTriggerAnalysisHandler(s.newsRepo, s.newsPipeline))
	}
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, findDocPath("internal/server/openapi.json"))
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, findDocPath("internal/server/swagger.html"))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func findDocPath(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	cwd, err := os.Getwd()
	if err == nil {
		path := filepath.Join(cwd, relPath)
		if exists(path) {
			return path
		}
	}
	root := cwd
	for i := 0; i < 5; i++ {
		parent := filepath.Dir(root)
		if parent == root || strings.TrimSpace(parent) == "" {
			break
		}
		root = parent
		path := filepath.Join(root, relPath)
		if exists(path) {
			return path
		}
	}
	return relPath
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

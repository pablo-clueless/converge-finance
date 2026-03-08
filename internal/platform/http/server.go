package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"converge-finance.com/m/internal/config"
	"converge-finance.com/m/internal/modules/ap"
	"converge-finance.com/m/internal/modules/ar"
	"converge-finance.com/m/internal/modules/close"
	"converge-finance.com/m/internal/modules/consol"
	"converge-finance.com/m/internal/modules/cost"
	"converge-finance.com/m/internal/modules/currency"
	"converge-finance.com/m/internal/modules/docs"
	"converge-finance.com/m/internal/modules/entity"
	"converge-finance.com/m/internal/modules/export"
	"converge-finance.com/m/internal/modules/fa"
	"converge-finance.com/m/internal/modules/fx"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/modules/ic"
	"converge-finance.com/m/internal/modules/segment"
	"converge-finance.com/m/internal/modules/user"
	"converge-finance.com/m/internal/modules/workflow"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

type Server struct {
	router         *chi.Mux
	server         *http.Server
	config         *config.Config
	db             *database.PostgresDB
	logger         *zap.Logger
	auditLogger    *audit.Logger
	jwtService     *auth.JWTService
	glModule       *gl.Module
	apModule       *ap.Module
	arModule       *ar.Module
	faModule       *fa.Module
	icModule       *ic.Module
	consolModule   *consol.Module
	costModule     *cost.Module
	closeModule    *close.Module
	fxModule       *fx.Module
	workflowModule *workflow.Module
	docsModule     *docs.Module
	exportModule   *export.Module
	segmentModule  *segment.Module
	currencyModule *currency.Module
	entityModule   *entity.Module
	userModule     *user.Module
}

func NewServer(cfg *config.Config, db *database.PostgresDB, logger *zap.Logger) *Server {
	eventStore := audit.NewPostgresEventStore(db.DB)
	s := &Server{
		router:      chi.NewRouter(),
		config:      cfg,
		db:          db,
		logger:      logger,
		auditLogger: audit.NewLogger(eventStore),
		jwtService:  auth.NewJWTService(cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry),
	}

	s.setupMiddleware()
	s.initModules()
	s.setupRoutes()

	return s
}

func (s *Server) initModules() {
	var err error

	s.glModule, err = gl.NewModule(gl.Config{
		DB:          s.db,
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize GL module", zap.Error(err))
	}

	s.apModule, err = ap.NewModule(ap.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize AP module", zap.Error(err))
	}

	s.arModule, err = ar.NewModule(ar.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize AR module", zap.Error(err))
	}

	s.faModule, err = fa.NewModule(fa.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize FA module", zap.Error(err))
	}

	s.icModule, err = ic.NewModule(ic.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize IC module", zap.Error(err))
	}

	s.consolModule, err = consol.NewModule(consol.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Consol module", zap.Error(err))
	}

	s.costModule, err = cost.NewModule(cost.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Cost module", zap.Error(err))
	}

	s.closeModule, err = close.NewModule(close.Config{
		DB:          s.db,
		GLAPI:       s.glModule.API(),
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Close module", zap.Error(err))
	}

	s.fxModule, err = fx.NewModule(fx.Config{
		DB:          s.db,
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize FX module", zap.Error(err))
	}

	s.workflowModule, err = workflow.NewModule(workflow.Config{
		DB:          s.db,
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Workflow module", zap.Error(err))
	}

	s.docsModule, err = docs.NewModule(docs.Config{
		DB:          s.db,
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Docs module", zap.Error(err))
	}

	s.exportModule, err = export.NewModule(export.Config{
		DB:          s.db,
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Export module", zap.Error(err))
	}

	s.segmentModule, err = segment.NewModule(segment.Config{
		DB:          s.db,
		AuditLogger: s.auditLogger,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Segment module", zap.Error(err))
	}

	s.currencyModule, err = currency.NewModule(currency.Config{
		DB:     s.db,
		Logger: s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Currency module", zap.Error(err))
	}

	s.entityModule, err = entity.NewModule(entity.Config{
		DB:     s.db,
		Logger: s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize Entity module", zap.Error(err))
	}

	s.userModule, err = user.NewModule(user.Config{
		DB:     s.db,
		Logger: s.logger,
	})
	if err != nil {
		s.logger.Error("Failed to initialize User module", zap.Error(err))
	}
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Entity-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (s *Server) setupRoutes() {
	s.router.Get("/", s.handleRoot)
	s.router.Get("/health", s.handleHealth)
	s.router.Get("/swagger", s.handleSwaggerUI)
	s.router.Get("/swagger/openapi.json", s.handleOpenAPISpec)
	s.router.Get("/redoc", s.handleRedoc)
	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.AuthMiddleware(s.jwtService))
		r.Use(s.entityContextMiddleware)
		if s.glModule != nil {
			s.glModule.RegisterRoutes(r)
		}
		if s.apModule != nil {
			s.apModule.RegisterRoutes(r)
		}
		if s.arModule != nil {
			s.arModule.RegisterRoutes(r)
		}
		if s.faModule != nil {
			s.faModule.RegisterRoutes(r)
		}
		if s.icModule != nil {
			s.icModule.RegisterRoutes(r)
		}
		if s.consolModule != nil {
			s.consolModule.RegisterRoutes(r)
		}
		if s.costModule != nil {
			s.costModule.RegisterRoutes(r)
		}
		if s.closeModule != nil {
			s.closeModule.RegisterRoutes(r)
		}
		if s.fxModule != nil {
			s.fxModule.RegisterRoutes(r)
		}
		if s.workflowModule != nil {
			s.workflowModule.RegisterRoutes(r)
		}
		if s.docsModule != nil {
			s.docsModule.RegisterRoutes(r)
		}
		if s.exportModule != nil {
			s.exportModule.RegisterRoutes(r)
		}
		if s.segmentModule != nil {
			s.segmentModule.RegisterRoutes(r)
		}

		if s.currencyModule != nil {
			s.currencyModule.RegisterRoutes(r)
		}
		if s.entityModule != nil {
			s.entityModule.RegisterRoutes(r)
		}
		if s.userModule != nil {
			s.userModule.RegisterRoutes(r)
		}
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"name":    "Converge Finance API",
		"version": "1.0.0",
		"docs": map[string]string{
			"swagger": "/swagger",
			"redoc":   "/redoc",
			"openapi": "/swagger/openapi.json",
		},
		"health":  "/health",
		"modules": []string{"gl", "ap", "ar", "fa", "fx", "ic", "close", "consol", "cost", "workflow", "docs", "export", "segment", "currency", "entity", "user"},
	})
}

func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Converge Finance API</title>
	<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
	<div id="swagger-ui"></div>
	<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	<script>
		SwaggerUIBundle({
			url: "/swagger/openapi.json",
			dom_id: '#swagger-ui',
		});
	</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *Server) handleRedoc(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Converge Finance API - Documentation</title>
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
	<style>
		body { margin: 0; padding: 0; }
	</style>
</head>
<body>
	<redoc spec-url="/swagger/openapi.json" expand-responses="200,201" hide-hostname="true"></redoc>
	<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "api/openapi/openapi.json")
}

func (s *Server) entityContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entityID := auth.GetEntityIDFromContext(r.Context())
		if entityID == "" {
			entityID = r.Header.Get("X-Entity-ID")
		}
		if entityID != "" {

			if err := s.db.SetEntityContext(r.Context(), entityID); err != nil {
				s.logger.Error("Failed to set entity context", zap.String("entity_id", entityID), zap.Error(err))
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.db.Health(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":   "unhealthy",
			"database": "down",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":   "healthy",
		"database": "up",
	})
}

func (s *Server) ListenAndServe(addr string) error {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// func respondError(w http.ResponseWriter, status int, message string) {
// 	respondJSON(w, status, map[string]string{"error": message})
// }

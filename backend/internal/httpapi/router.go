package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Dependencies struct {
	AuthService *service.Auth
	RFQService  *service.RFQs
	BidService  *service.Bids
	Tokens      *auth.TokenManager
}

func NewRouter(dependencies Dependencies) http.Handler {
	handlers := newHandlers(dependencies.AuthService, dependencies.RFQService, dependencies.BidService)
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Get("/healthz", health)
	router.Post("/api/v1/auth/login", handlers.login)
	router.Group(func(protected chi.Router) {
		protected.Use(authenticate(dependencies.Tokens))
		protected.Get("/api/v1/auth/me", handlers.me)
		protected.Get("/api/v1/rfqs", handlers.listRFQs)
		protected.Post("/api/v1/rfqs", handlers.createRFQ)
		protected.Get("/api/v1/rfqs/{rfq_id}", handlers.getRFQ)
		protected.Post("/api/v1/rfqs/{rfq_id}/bids", handlers.submitBid)
		protected.Get("/api/v1/rfqs/{rfq_id}/ranking", handlers.getRanking)
		protected.Get("/api/v1/rfqs/{rfq_id}/bids", handlers.listBids)
	})
	return router
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

package api

import (
	"chat-app-backend/internal/api/middleware"
	"chat-app-backend/internal/queue"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type apiFunc func(http.ResponseWriter, *http.Request) error

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func (s *APIServer) MakeHTTPHandleFunc(f apiFunc, authMiddleware ...middleware.Middleware) http.HandlerFunc {
	corsConfig := middleware.CORSConfig{
		AllowedOrigins:   []string{"http://localhost:3000", "https://heyholi.pl", "https://api.heyholi.pl", "https://new-kosmetolog.heyholi.pl"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "OPTIONS", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "X-Requested-With", "Authorization"},
		AllowCredentials: true,
	}

	baseHandler := func(w http.ResponseWriter, r *http.Request) {
		errc := make(chan error, 1)

		job := queue.Job{
			Fn: func() error {
				err := f(w, r)
				return err
			},
			Errc: errc,
		}

		s.requestQueueManager.EnqueueJob(job)

		err := <-errc
		if err != nil {
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				fmt.Println(httpErr.ErrorLog)
				WriteJSON(w, httpErr.StatusCode, ApiError{Error: httpErr.Message})
			} else {
				WriteJSON(w, http.StatusInternalServerError, ApiError{Error: "Internal server error"})
			}
		}
	}

	middlewares := []middleware.Middleware{
		middleware.CORS(corsConfig),
		middleware.Logging(),
	}

	finalHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if len(authMiddleware) > 0 {
			authHandler := baseHandler
			for _, m := range authMiddleware {
				authHandler = m(authHandler)
			}
			authHandler(w, r)
		} else {
			baseHandler(w, r)
		}
	}

	return middleware.Chain(finalHandler, middlewares...)
}

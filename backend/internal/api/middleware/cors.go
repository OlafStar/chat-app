package middleware

import (
	"chat-app-backend/utils"
	"net/http"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

func CORS(config CORSConfig) Middleware {
	return func(f http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Log the incoming request details.
			// log.Printf("Received request: method=%s, path=%s, origin=%s", r.Method, r.URL.Path, r.Header.Get("Origin"))

			origin := r.Header.Get("Origin")
			allowedOrigin := ""

			// Determine the allowed origin based on config.
			for _, o := range config.AllowedOrigins {
				if o == "*" {
					if config.AllowCredentials {
						allowedOrigin = origin
						// log.Printf("Wildcard '*' allowed with credentials. Using request origin: %s", origin)
					} else {
						allowedOrigin = "*"
						// log.Printf("Wildcard '*' allowed without credentials. Using '*'")
					}
					break
				} else if o == origin {
					allowedOrigin = o
					// log.Printf("Request origin %s matched allowed origin: %s", origin, o)
					break
				}
			}

			// Set the CORS headers if an allowed origin was found.
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				// log.Printf("Set header: Access-Control-Allow-Origin: %s", allowedOrigin)

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					// log.Printf("Set header: Access-Control-Allow-Credentials: true")
				}
				methods := utils.StringJoin(config.AllowedMethods, ", ")
				w.Header().Set("Access-Control-Allow-Methods", methods)
				// log.Printf("Set header: Access-Control-Allow-Methods: %s", methods)

				headers := utils.StringJoin(config.AllowedHeaders, ", ")
				w.Header().Set("Access-Control-Allow-Headers", headers)
				// log.Printf("Set header: Access-Control-Allow-Headers: %s", headers)
			}
			// else {
			// log.Printf("No allowed origin found for request origin: %s", origin)
			// }

			// Handle preflight OPTIONS requests immediately.
			if r.Method == http.MethodOptions {
				if allowedOrigin != "" {
					// log.Printf("OPTIONS request: Allowed origin present. Responding with 200 OK.")
					w.WriteHeader(http.StatusOK)
				} else {
					// log.Printf("OPTIONS request: No allowed origin. Responding with 403 Forbidden.")
					w.WriteHeader(http.StatusForbidden)
				}
				return
			}

			// For non-OPTIONS requests, call the actual handler.
			// log.Printf("Non-OPTIONS request: Passing request to the handler.")
			f(w, r)
			// log.Printf("Request processing complete for path: %s", r.URL.Path)
		}
	}
}

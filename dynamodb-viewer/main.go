package main

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/olaf/chat-app/dynamodb-viewer/internal/dynamo"
)

//go:embed web/static/*
var staticFiles embed.FS

func main() {
	cfg := loadConfig()

	ctx := context.Background()
	dyn, err := dynamo.New(ctx, dynamo.Config{
		Region:       cfg.Region,
		Endpoint:     cfg.Endpoint,
		AccessKey:    cfg.AccessKey,
		SecretKey:    cfg.SecretKey,
		SessionToken: cfg.SessionToken,
	})
	if err != nil {
		log.Fatalf("failed to init dynamodb client: %v", err)
	}

	srv := &apiServer{
		dynamo:       dyn,
		defaultLimit: cfg.DefaultLimit,
		maxLimit:     cfg.MaxLimit,
		region:       cfg.Region,
		endpoint:     cfg.Endpoint,
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	staticFS, err := fs.Sub(staticFiles, "web/static")
	if err != nil {
		log.Fatalf("failed to prepare static assets: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", fileServer)

	server := &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      logRequests(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("DynamoDB viewer listening on %s (region=%s endpoint=%s)", cfg.BindAddr, cfg.Region, cfg.Endpoint)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

type viewerConfig struct {
	BindAddr     string
	Region       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	SessionToken string
	DefaultLimit int32
	MaxLimit     int32
}

func loadConfig() viewerConfig {
	return viewerConfig{
		BindAddr:     getEnv("VIEWER_BIND_ADDR", ":4100"),
		Region:       getEnv("AWS_REGION", "eu-central-1"),
		Endpoint:     os.Getenv("DYNAMODB_ENDPOINT"),
		AccessKey:    getEnv("AWS_ACCESS_KEY_ID", getEnv("AWS_ID", "local")),
		SecretKey:    getEnv("AWS_SECRET_ACCESS_KEY", getEnv("AWS_SECRET", "local")),
		SessionToken: getEnv("AWS_SESSION_TOKEN", ""),
		DefaultLimit: int32(getEnvAsInt("VIEWER_DEFAULT_LIMIT", 25)),
		MaxLimit:     int32(getEnvAsInt("VIEWER_MAX_LIMIT", 200)),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return fallback
}

type apiServer struct {
	dynamo       *dynamo.Service
	defaultLimit int32
	maxLimit     int32
	region       string
	endpoint     string
}

func (a *apiServer) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/config", a.handleConfig)
	mux.HandleFunc("/api/tables", a.handleTables)
	mux.HandleFunc("/api/tables/", a.handleTable)
}

func (a *apiServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"region":       a.region,
		"endpoint":     a.endpoint,
		"defaultLimit": a.defaultLimit,
		"maxLimit":     a.maxLimit,
	})
}

func (a *apiServer) handleTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/api/tables" {
		http.NotFound(w, r)
		return
	}

	names, err := a.dynamo.ListTables(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"tables": names})
}

func (a *apiServer) handleTable(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/tables/") {
		http.NotFound(w, r)
		return
	}

	remainder := strings.TrimPrefix(r.URL.Path, "/api/tables/")
	parts := strings.SplitN(remainder, "/", 3)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	tableName, err := urlPathUnescape(parts[0])
	if err != nil {
		writeErrorWithStatus(w, http.StatusBadRequest, fmt.Errorf("invalid table name: %w", err))
		return
	}

	action := parts[1]

	switch action {
	case "items":
		a.handleItems(w, r, tableName)
	case "meta":
		a.handleMeta(w, r, tableName)
	default:
		http.NotFound(w, r)
	}
}

func (a *apiServer) handleMeta(w http.ResponseWriter, r *http.Request, table string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	meta, err := a.dynamo.DescribeTable(r.Context(), table)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"table": meta})
}

func (a *apiServer) handleItems(w http.ResponseWriter, r *http.Request, table string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := a.defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = int32(parsed)
		}
	}
	if limit <= 0 {
		limit = a.defaultLimit
	}
	if limit > a.maxLimit {
		limit = a.maxLimit
	}

	var startKey map[string]types.AttributeValue
	if token := r.URL.Query().Get("startKey"); token != "" {
		key, err := decodeKey(token)
		if err != nil {
			writeErrorWithStatus(w, http.StatusBadRequest, err)
			return
		}
		startKey = key
	}

	result, err := a.dynamo.ScanTable(r.Context(), table, limit, startKey)
	if err != nil {
		writeError(w, err)
		return
	}

	nextToken, err := encodeKey(result.LastEvaluatedKey)
	if err != nil {
		writeError(w, err)
		return
	}

	payload := map[string]interface{}{
		"items":         result.Items,
		"count":         result.Count,
		"scannedCount":  result.ScannedCount,
		"nextPageToken": nextToken,
	}

	if result.LastEvaluatedKey != nil {
		payload["lastEvaluatedKey"] = result.LastEvaluatedKey
	}

	writeJSON(w, http.StatusOK, payload)
}

func decodeKey(token string) (map[string]types.AttributeValue, error) {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("decode start key: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal start key: %w", err)
	}
	return dynamo.MarshalInterfaceMap(raw)
}

func encodeKey(key map[string]interface{}) (string, error) {
	if key == nil {
		return "", nil
	}
	data, err := json.Marshal(key)
	if err != nil {
		return "", fmt.Errorf("marshal key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func urlPathUnescape(value string) (string, error) {
	return url.PathUnescape(value)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	writeErrorWithStatus(w, http.StatusInternalServerError, err)
}

func writeErrorWithStatus(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

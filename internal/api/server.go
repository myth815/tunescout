package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/federation"
	"github.com/myth815/tunescout/internal/model"
	"github.com/myth815/tunescout/internal/provider"
)

type Server struct {
	engine    *federation.Engine
	providers []provider.Provider
	config    config.Config
	logger    *slog.Logger
	mux       *http.ServeMux
}

func New(engine *federation.Engine, providers []provider.Provider, cfg config.Config, logger *slog.Logger) http.Handler {
	server := &Server{engine: engine, providers: providers, config: cfg, logger: logger, mux: http.NewServeMux()}
	server.routes()
	return server.securityHeaders(server.authentication(server.logging(server.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /v1/providers", s.listProviders)
	s.mux.HandleFunc("GET /v1/search", s.searchGET)
	s.mux.HandleFunc("POST /v1/search", s.searchPOST)
	s.mux.HandleFunc("GET /v1/entities/{entity_ref}", s.entity)
}

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"status": "ok", "service": "tunescout", "time": time.Now().UTC()})
}

func (s *Server) listProviders(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"domain": "music", "providers": s.engine.Providers()})
}

func (s *Server) searchGET(writer http.ResponseWriter, request *http.Request) {
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	searchRequest := model.SearchRequest{
		Query: request.URL.Query().Get("q"), Types: splitCSV(request.URL.Query().Get("types")), Limit: limit,
		Locale: request.URL.Query().Get("locale"), Strategy: model.Strategy{Providers: splitCSV(request.URL.Query().Get("providers")), Region: request.URL.Query().Get("region")},
	}
	s.runSearch(writer, request, searchRequest, "")
}

func (s *Server) searchPOST(writer http.ResponseWriter, request *http.Request) {
	contentType := request.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		searchRequest, audioPath, err := s.parseMultipart(request)
		if err != nil {
			writeError(writer, http.StatusBadRequest, "invalid_multipart", err.Error())
			return
		}
		if audioPath != "" {
			defer os.Remove(audioPath)
		}
		s.runSearch(writer, request, searchRequest, audioPath)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	defer request.Body.Close()
	var searchRequest model.SearchRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&searchRequest); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	s.runSearch(writer, request, searchRequest, "")
}

func (s *Server) runSearch(writer http.ResponseWriter, request *http.Request, searchRequest model.SearchRequest, audioPath string) {
	searchRequest.AudioPath = audioPath
	response, err := s.engine.Search(request.Context(), searchRequest)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "search_failed", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func (s *Server) parseMultipart(request *http.Request) (model.SearchRequest, string, error) {
	if err := request.ParseMultipartForm(s.config.MaxUploadBytes); err != nil {
		return model.SearchRequest{}, "", fmt.Errorf("parse multipart form: %w", err)
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	manifest := request.FormValue("request")
	var searchRequest model.SearchRequest
	if strings.TrimSpace(manifest) != "" {
		decoder := json.NewDecoder(strings.NewReader(manifest))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&searchRequest); err != nil {
			return model.SearchRequest{}, "", fmt.Errorf("decode request part: %w", err)
		}
	}
	file, header, err := request.FormFile("audio")
	if errors.Is(err, http.ErrMissingFile) {
		if strings.TrimSpace(searchRequest.Query) == "" && len(searchRequest.Inputs) == 0 {
			return model.SearchRequest{}, "", errors.New("request or audio part is required")
		}
		return searchRequest, "", nil
	}
	if err != nil {
		return model.SearchRequest{}, "", fmt.Errorf("open audio part: %w", err)
	}
	defer file.Close()
	path, err := saveUpload(file, header, s.config.MaxUploadBytes)
	if err != nil {
		return model.SearchRequest{}, "", err
	}
	return searchRequest, path, nil
}

func saveUpload(file multipart.File, header *multipart.FileHeader, maximum int64) (string, error) {
	extension := filepath.Ext(filepath.Base(header.Filename))
	target, err := os.CreateTemp("", "tunescout-audio-*"+extension)
	if err != nil {
		return "", err
	}
	path := target.Name()
	succeeded := false
	defer func() {
		target.Close()
		if !succeeded {
			os.Remove(path)
		}
	}()
	written, err := io.Copy(target, io.LimitReader(file, maximum+1))
	if err != nil {
		return "", err
	}
	if written > maximum {
		return "", fmt.Errorf("audio upload exceeds %d bytes", maximum)
	}
	if err := target.Close(); err != nil {
		return "", err
	}
	succeeded = true
	return path, nil
}

func (s *Server) entity(writer http.ResponseWriter, request *http.Request) {
	include := splitCSV(request.URL.Query().Get("include"))
	response, err := s.engine.Lookup(request.Context(), request.PathValue("entity_ref"), include)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "entity_lookup_failed", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func (s *Server) authentication(next http.Handler) http.Handler {
	if s.config.APIKey == "" {
		return next
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/healthz" {
			next.ServeHTTP(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer "+s.config.APIKey {
			writeError(writer, http.StatusUnauthorized, "unauthorized", "a valid bearer token is required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(writer, request)
		s.logger.Info("request", "method", request.Method, "path", request.URL.Path, "elapsed_ms", time.Since(started).Milliseconds())
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(writer, request)
	})
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

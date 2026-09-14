package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hovergo/rqbin/internal/config"
	"github.com/hovergo/rqbin/internal/store"
)

const maxRequestsOnPage = 50

// Handler обслуживает HTTP-эндпоинты приложения
type Handler struct {
	cfg       config.Config
	store     *store.Store
	templates *template.Template
	log       *slog.Logger
}

func New(cfg config.Config, st *store.Store, templatesDir string, log *slog.Logger) (*Handler, error) {
	pattern := filepath.Join(templatesDir, "*.html")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatTime": formatTime,
		"join":       strings.Join,
		"headerKeys": headerKeys,
	}).ParseGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{
		cfg:       cfg,
		store:     st,
		templates: tmpl,
		log:       log,
	}, nil
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("POST /bins", h.createBin)
	mux.HandleFunc("GET /b/{id}", h.showBin)
	mux.HandleFunc("GET /b/{id}/requests", h.binRequestsPartial)
	mux.HandleFunc("/b/{id}", h.captureRequest)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	return mux
}

type binPageData struct {
	Bin         store.Bin
	EndpointURL string
	Requests    []store.Request
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	h.render(w, "home.html", nil)
}

func (h *Handler) createBin(w http.ResponseWriter, r *http.Request) {
	id, err := newBinID()
	if err != nil {
		h.log.Error("generate bin id", "err", err)
		http.Error(w, "failed to create bin", http.StatusInternalServerError)
		return
	}

	if _, err := h.store.CreateBin(r.Context(), id); err != nil {
		h.log.Error("create bin", "err", err)
		http.Error(w, "failed to create bin", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/b/"+id, http.StatusSeeOther)
}

func (h *Handler) showBin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bin, err := h.store.GetBin(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.log.Error("get bin", "err", err, "bin_id", id)
		http.Error(w, "failed to load bin", http.StatusInternalServerError)
		return
	}

	requests, err := h.store.ListRequests(r.Context(), id, maxRequestsOnPage)
	if err != nil {
		h.log.Error("list requests", "err", err, "bin_id", id)
		http.Error(w, "failed to load requests", http.StatusInternalServerError)
		return
	}

	h.render(w, "bin.html", binPageData{
		Bin:         bin,
		EndpointURL: strings.TrimRight(h.cfg.PublicBaseURL, "/") + "/b/" + bin.ID,
		Requests:    requests,
	})
}

func (h *Handler) binRequestsPartial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.store.GetBin(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.log.Error("get bin", "err", err, "bin_id", id)
		http.Error(w, "failed to load bin", http.StatusInternalServerError)
		return
	}

	requests, err := h.store.ListRequests(r.Context(), id, maxRequestsOnPage)
	if err != nil {
		h.log.Error("list requests", "err", err, "bin_id", id)
		http.Error(w, "failed to load requests", http.StatusInternalServerError)
		return
	}

	h.render(w, "requests.html", binPageData{Requests: requests})
}

func (h *Handler) captureRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.store.GetBin(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.log.Error("get bin", "err", err, "bin_id", id)
		http.Error(w, "failed to load bin", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, h.cfg.RequestBodyLimit+1))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > h.cfg.RequestBodyLimit {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	headers := cloneHeaders(r.Header)
	saved, err := h.store.SaveRequest(r.Context(), store.Request{
		BinID:       id,
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryString: r.URL.RawQuery,
		Headers:     headers,
		Body:        string(body),
		RemoteAddr:  clientIP(r),
	})
	if err != nil {
		h.log.Error("save request", "err", err, "bin_id", id)
		http.Error(w, "failed to save request", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"ok":true,"id":%d}`+"\n", saved.ID)
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.log.Error("render template", "err", err, "template", name)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func newBinID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func cloneHeaders(src http.Header) map[string][]string {
	dst := make(map[string][]string, len(src))
	for k, vals := range src {
		copied := make([]string, len(vals))
		copy(copied, vals)
		dst[k] = copied
	}
	return dst
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}

func headerKeys(headers map[string][]string) []string {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

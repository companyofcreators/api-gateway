package proxy

import (
	"io"
	"net/http"
	"strings"

	"github.com/gookit/slog"
)

type ReverseProxy struct {
	routes     map[string]string // prefix -> target URL
	rewrite    map[string]string // prefix -> internal rewrite path
	httpClient *http.Client
}

func NewReverseProxy(routes map[string]string, httpClient *http.Client) *ReverseProxy {
	rewrite := map[string]string{
		"/api/v1/auth/":          "/api/v1/auth/",
		"/api/v1/users/":         "/internal/users/",
		"/api/v1/masters/":       "/internal/masters/",
		"/api/v1/orders/":        "/internal/orders/",
		"/api/v1/categories/":    "/internal/categories/",
		"/api/v1/reviews/":       "/internal/reviews/",
		"/api/v1/complaints/":    "/internal/complaints/",
		"/api/v1/offers/":        "/internal/offers/",
		"/api/v1/chat/":          "/internal/chats/",
		"/api/v1/chats/":         "/internal/chats/",
		"/api/v1/files/":         "/internal/files/",
		"/api/v1/notifications/": "/internal/notifications/",
		"/api/v1/admin/":         "/api/v1/admin/",
	}
	return &ReverseProxy{routes: routes, rewrite: rewrite, httpClient: httpClient}
}

// ProxyTo creates a handler that proxies requests to a specific target service,
// rewriting the matched path prefix to the target prefix. The matchPrefix must end
// with "/" and is stripped from the request path; rewritePrefix is prepended instead.
// If rewritePrefix equals matchPrefix, the path is forwarded unchanged.
func (rp *ReverseProxy) ProxyTo(targetURL, matchPrefix, rewritePrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		rest := strings.TrimPrefix(path, matchPrefix)
		newPath := strings.TrimRight(rewritePrefix+rest, "/")
		proxyURL := targetURL + newPath
		if q := r.URL.RawQuery; q != "" {
			proxyURL += "?" + q
		}

		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL, r.Body)
		if err != nil {
			slog.Error("proxy create request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if _, werr := w.Write([]byte(`{"error":"внутренняя ошибка","message":"не удалось создать прокси-запрос"}`)); werr != nil {
				slog.Debug("write proxy error response error", "error", werr)
			}
			return
		}

		for key, values := range r.Header {
			if strings.EqualFold(key, "Cookie") {
				continue
			}
			for _, v := range values {
				proxyReq.Header.Add(key, v)
			}
		}

		resp, err := rp.httpClient.Do(proxyReq)
		if err != nil {
			slog.Error("proxy request failed", "error", err, "target", proxyURL)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			if _, werr := w.Write([]byte(`{"error":"шлюз недоступен","message":"внутренний сервис не отвечает"}`)); werr != nil {
				slog.Debug("write bad gateway response error", "error", werr)
			}
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(resp.StatusCode)
		if resp.Body != nil {
			if _, err := io.Copy(w, resp.Body); err != nil {
				slog.Debug("proxy response copy error", "error", err)
			}
		}
	}
}

func (rp *ReverseProxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		var target string
		var matchedPrefix string
		for prefix, t := range rp.routes {
			if strings.HasPrefix(path, prefix) && len(prefix) > len(matchedPrefix) {
				target, matchedPrefix = t, prefix
			}
		}

		if target == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte(`{"error":"не найдено","message":"маршрут не найден"}`)); err != nil {
				slog.Debug("write not found response error", "error", err)
			}
			return
		}

		// Rewrite path.
		rw := rp.rewrite[matchedPrefix]
		if rw == "" {
			rw = matchedPrefix
		}
		rest := strings.TrimPrefix(path, matchedPrefix)
		newPath := strings.TrimRight(rw+rest, "/")

		// Build proxied request, preserving query parameters.
		proxyURL := target + newPath
		if q := r.URL.RawQuery; q != "" {
			proxyURL += "?" + q
		}
		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL, r.Body)
		if err != nil {
			slog.Error("proxy create request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if _, werr := w.Write([]byte(`{"error":"внутренняя ошибка","message":"не удалось создать прокси-запрос"}`)); werr != nil {
				slog.Debug("write proxy error response error", "error", werr)
			}
			return
		}

		// Copy headers to backend.
		// Cookie is forwarded only to auth-service (which needs refresh_token cookie).
		for key, values := range r.Header {
			if strings.EqualFold(key, "Cookie") && !strings.HasPrefix(matchedPrefix, "/api/v1/auth/") {
				continue
			}
			for _, v := range values {
				proxyReq.Header.Add(key, v)
			}
		}

		resp, err := rp.httpClient.Do(proxyReq)
		if err != nil {
			slog.Error("proxy request failed", "error", err, "target", target+newPath)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			if _, werr := w.Write([]byte(`{"error":"шлюз недоступен","message":"внутренний сервис не отвечает"}`)); werr != nil {
				slog.Debug("write bad gateway response error", "error", werr)
			}
			return
		}
		defer resp.Body.Close()

		// Copy response.
		for key, values := range resp.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(resp.StatusCode)
		if resp.Body != nil {
			if _, err := io.Copy(w, resp.Body); err != nil {
				slog.Debug("proxy response copy error", "error", err)
			}
		}
	})
}

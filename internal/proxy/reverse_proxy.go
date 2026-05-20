package proxy

import (
	"net/http"
	"strings"

	"github.com/gookit/slog"
)

type ReverseProxy struct {
	routes  map[string]string // prefix -> target URL
	rewrite map[string]string // prefix -> internal rewrite path
}

func NewReverseProxy(routes map[string]string) *ReverseProxy {
	rewrite := map[string]string{
		"/api/v1/auth/":          "/api/v1/auth/",
		"/api/v1/users/":         "/internal/users/",
		"/api/v1/masters/":       "/internal/masters/",
		"/api/v1/orders/":        "/internal/orders/",
		"/api/v1/categories/":    "/internal/categories/",
		"/api/v1/reviews/":       "/internal/reviews/",
		"/api/v1/complaints/":    "/internal/complaints/",
		"/api/v1/offers/":        "/internal/offers/",
		"/api/v1/chat/":          "/internal/chat/",
		"/api/v1/chats/":         "/internal/chats/",
		"/api/v1/files/":         "/internal/files/",
		"/api/v1/notifications/": "/internal/notifications/",
		"/api/v1/admin/":         "/api/v1/admin/",
	}
	return &ReverseProxy{routes: routes, rewrite: rewrite}
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
			_, _ = w.Write([]byte(`{"error":"не найдено","message":"маршрут не найден"}`))
			return
		}

		// Rewrite path.
		rw := rp.rewrite[matchedPrefix]
		if rw == "" {
			rw = matchedPrefix
		}
		rest := strings.TrimPrefix(path, matchedPrefix)
		newPath := strings.TrimRight(rw+rest, "/")

		// Build proxied request.
		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, target+newPath, r.Body)
		if err != nil {
			slog.Error("proxy create request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"внутренняя ошибка","message":"не удалось создать прокси-запрос"}`))
			return
		}

		// Copy headers (X-User-Id, X-User-Email, X-User-Role, etc.).
		for key, values := range r.Header {
			for _, v := range values {
				proxyReq.Header.Add(key, v)
			}
		}

		resp, err := http.DefaultClient.Do(proxyReq)
		if err != nil {
			slog.Error("proxy request failed", "error", err, "target", target+newPath)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"шлюз недоступен","message":"внутренний сервис не отвечает"}`))
			return
		}
		defer resp.Body.Close()

		// Copy response.
		for key, values := range resp.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(resp.StatusCode)
		if resp.Body != nil {
			buf := make([]byte, 32*1024)
			for {
				n, err := resp.Body.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
				}
				if err != nil {
					break
				}
			}
		}
	})
}

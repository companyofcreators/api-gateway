package middleware

import (
	"encoding/json"
	"net/http"
)

// RequireRoles returns middleware that checks whether the request's
// X-User-Role header (set by the Auth middleware) matches one of the
// allowed roles. If the role is not in the allowed list, it returns 403.
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(RequireRoles("admin"))
//	    r.Delete("/api/v1/admin/users/{id}", handler)
//	})
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := r.Header.Get("X-User-Role")

			if userRole == "" {
				writeForbiddenError(w, r, "отсутствует информация о роли", nil)
				return
			}

			if !allowed[userRole] {
				writeForbiddenError(w, r, "недостаточно прав", &userRole)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeForbiddenError(w http.ResponseWriter, r *http.Request, message string, role *string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	resp := map[string]string{
		"error":      message,
		"request_id": GetRequestID(r.Context()),
	}

	if role != nil {
		resp["role"] = *role
	}

	_ = json.NewEncoder(w).Encode(resp)
}

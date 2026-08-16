package httpapi

import (
	"context"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/iam"
)

type authHandler struct {
	users         *iam.Store
	ttl           time.Duration
	secureCookies bool
	limiter       loginLimiter
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts []time.Time
}

func newAuthHandler(users *iam.Store, ttl time.Duration, secureCookies bool) *authHandler {
	return &authHandler{users: users, ttl: ttl, secureCookies: secureCookies}
}

func (handler *authHandler) login(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	address, err := mail.ParseAddress(body.Email)
	if err != nil || address.Address != body.Email || len(body.Email) > 254 || len(body.Password) == 0 || len(body.Password) > 72 {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "A valid email and password are required.")
		return
	}
	if !handler.limiter.Allow(time.Now()) {
		writeProblem(w, http.StatusTooManyRequests, "https://snap.local/problems/rate-limit", "Too many attempts", "Try again in a minute.")
		return
	}
	user, err := handler.users.Authenticate(request.Context(), body.Email, body.Password)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "https://snap.local/problems/auth", "Unauthorized", "Invalid credentials.")
		return
	}
	session, err := handler.users.CreateSession(request.Context(), user.ID, handler.ttl)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not create session.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "snap_session",
		Value:    session,
		Path:     "/",
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(handler.ttl.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": userJSON(user)})
}

func (handler *authHandler) logout(w http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie("snap_session"); err == nil {
		handler.users.RevokeSession(request.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "snap_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *authHandler) me(w http.ResponseWriter, request *http.Request) {
	user := currentUser(request)
	writeJSON(w, http.StatusOK, userJSON(user))
}

type userContextKey struct{}

func currentUser(request *http.Request) iam.User {
	user, _ := request.Context().Value(userContextKey{}).(iam.User)
	return user
}

func (handler *authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie("snap_session")
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "https://snap.local/problems/auth", "Unauthorized", "Sign in required.")
			return
		}
		user, err := handler.users.LookupSession(request.Context(), cookie.Value)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "https://snap.local/problems/auth", "Unauthorized", "Sign in required.")
			return
		}
		next.ServeHTTP(w, request.WithContext(context.WithValue(request.Context(), userContextKey{}, user)))
	})
}

func userJSON(user iam.User) map[string]string {
	return map[string]string{
		"id": user.ID.String(), "email": user.Email, "name": user.DisplayName, "role": user.Role,
	}
}

func (limiter *loginLimiter) Allow(now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	windowStart := now.Add(-time.Minute)
	kept := limiter.attempts[:0]
	for _, attempt := range limiter.attempts {
		if attempt.After(windowStart) {
			kept = append(kept, attempt)
		}
	}
	limiter.attempts = kept
	// ponytail: one bucket fits this single-admin portal; use a distributed limiter before horizontal scaling.
	if len(limiter.attempts) >= 5 {
		return false
	}
	limiter.attempts = append(limiter.attempts, now)
	return true
}

package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/iam"
)

type pendingMFA struct {
	UserID uuid.UUID
	Secret string
	Expiry time.Time
}

type authHandler struct {
	users   *iam.Store
	ttl     time.Duration
	pending map[string]pendingMFA
	mu      sync.Mutex
}

func newAuthHandler(users *iam.Store, ttl time.Duration) *authHandler {
	return &authHandler{users: users, ttl: ttl, pending: map[string]pendingMFA{}}
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
	user, err := handler.users.Authenticate(request.Context(), body.Email, body.Password)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "https://snap.local/problems/auth", "Unauthorized", "Invalid credentials.")
		return
	}
	token := newOpaqueToken()
	handler.mu.Lock()
	challenge := pendingMFA{UserID: user.ID, Expiry: time.Now().Add(10 * time.Minute)}
	if !user.HasTOTP {
		secret, url, qr, err := handler.users.CreateTOTP(user.Email)
		if err != nil {
			handler.mu.Unlock()
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not start MFA.")
			return
		}
		challenge.Secret = secret
		handler.pending[token] = challenge
		handler.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"mfa_required": true, "mfa_setup": true, "mfa_token": token, "otpauth_url": url, "otpauth_qr": qr,
		})
		return
	}
	handler.pending[token] = challenge
	handler.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"mfa_required": true, "mfa_setup": false, "mfa_token": token})
}

func (handler *authHandler) verifyMFA(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Token string `json:"mfa_token"`
		Code  string `json:"code"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	handler.mu.Lock()
	challenge, ok := handler.pending[body.Token]
	if !ok || time.Now().After(challenge.Expiry) {
		handler.mu.Unlock()
		writeProblem(w, http.StatusUnauthorized, "https://snap.local/problems/auth", "Unauthorized", "MFA challenge expired.")
		return
	}
	delete(handler.pending, body.Token)
	handler.mu.Unlock()

	var err error
	if challenge.Secret != "" {
		err = handler.users.ConfirmTOTP(request.Context(), challenge.UserID, challenge.Secret, body.Code)
	} else {
		err = handler.users.VerifyTOTP(request.Context(), challenge.UserID, body.Code)
	}
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "https://snap.local/problems/auth", "Unauthorized", "Invalid code.")
		return
	}
	session, err := handler.users.CreateSession(request.Context(), challenge.UserID, handler.ttl)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not create session.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "snap_session",
		Value:    session,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(handler.ttl.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *authHandler) me(w http.ResponseWriter, request *http.Request) {
	user := currentUser(request)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": user.ID.String(), "email": user.Email, "name": user.DisplayName, "role": user.Role,
	})
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

func newOpaqueToken() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)
}

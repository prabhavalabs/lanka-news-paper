package httpapi

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/newsletter"
)

type newsletterAdminHandler struct {
	repository newsletter.Repository
}

func (handler newsletterAdminHandler) settings(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	if request.Method == http.MethodGet {
		settings, err := handler.repository.GetSettings(request.Context())
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not load newsletter settings.")
			return
		}
		writeJSON(w, http.StatusOK, settings)
		return
	}
	var input newsletter.Settings
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	settings, err := handler.repository.UpdateSettings(request.Context(), input, currentUser(request).ID)
	if errors.Is(err, newsletter.ErrInvalidSettings) {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Check the timezone, delivery hour, story limits, and template lengths.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not update newsletter settings.")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (handler newsletterAdminHandler) subscribers(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	if request.Method == http.MethodGet {
		items, err := handler.repository.ListSubscribers(request.Context())
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not load newsletter recipients.")
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	var input newsletter.SubscriberInput
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	item, err := handler.repository.CreateSubscriber(request.Context(), input, currentUser(request).ID)
	if err != nil {
		writeNewsletterProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (handler newsletterAdminHandler) subscriber(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	id, err := uuid.Parse(request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Recipient id must be a UUID.")
		return
	}
	if request.Method == http.MethodDelete {
		if err := handler.repository.DeleteSubscriber(request.Context(), id, currentUser(request).ID); err != nil {
			writeNewsletterProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	var input newsletter.SubscriberInput
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	item, err := handler.repository.UpdateSubscriber(request.Context(), id, input, currentUser(request).ID)
	if err != nil {
		writeNewsletterProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func writeNewsletterProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, newsletter.ErrEmailExists):
		writeProblem(w, http.StatusConflict, "https://snap.local/problems/conflict", "Email already added", err.Error())
	case errors.Is(err, newsletter.ErrSubscriberMissing):
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", err.Error())
	case errors.Is(err, newsletter.ErrConsentRequired), errors.Is(err, newsletter.ErrInvalidEmail),
		errors.Is(err, newsletter.ErrInvalidName), errors.Is(err, newsletter.ErrInvalidStatus),
		errors.Is(err, newsletter.ErrInvalidSettings):
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
	default:
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not update the newsletter recipient.")
	}
}

type newsletterUnsubscribeHandler struct {
	repository newsletter.Repository
}

func (handler newsletterUnsubscribeHandler) unsubscribe(w http.ResponseWriter, request *http.Request) {
	token, err := uuid.Parse(request.PathValue("token"))
	if err != nil {
		http.NotFound(w, request)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if request.Method == http.MethodGet {
		_ = unsubscribePage.Execute(w, map[string]string{"Action": fmt.Sprintf("/api/v1/newsletter/unsubscribe/%s", token)})
		return
	}
	if err := handler.repository.Unsubscribe(request.Context(), token); err != nil && !errors.Is(err, newsletter.ErrSubscriberMissing) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><p>Could not update your newsletter preference.</p>"))
		return
	}
	_ = unsubscribeDonePage.Execute(w, nil)
}

var unsubscribePage = template.Must(template.New("unsubscribe").Parse(`<!doctype html>
<html lang="si"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>පුවත් සංග්‍රහයෙන් ඉවත් වන්න</title>
<body style="margin:0;background:#f6f5f2;color:#171717;font-family:'Noto Sans Sinhala','Nirmala UI',sans-serif">
<main style="max-width:560px;margin:12vh auto;padding:32px;background:white;border:1px solid #dedbd4">
<h1 style="font-size:26px">පුවත් සංග්‍රහයෙන් ඉවත් වන්න</h1>
<p style="font-size:17px;line-height:1.7">දිනපතා උදෑසන පුවත් සංග්‍රහය ලැබීම නතර කිරීමට පහත බොත්තම ඔබන්න.</p>
<form method="post" action="{{.Action}}"><button style="border:0;background:#171717;color:white;padding:12px 18px;font:inherit;cursor:pointer">ලැබීම නතර කරන්න</button></form>
</main></body></html>`))

var unsubscribeDonePage = template.Must(template.New("unsubscribed").Parse(`<!doctype html>
<html lang="si"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>ඉවත් වීම සාර්ථකයි</title>
<body style="margin:0;background:#f6f5f2;color:#171717;font-family:'Noto Sans Sinhala','Nirmala UI',sans-serif">
<main style="max-width:560px;margin:12vh auto;padding:32px;background:white;border:1px solid #dedbd4">
<h1 style="font-size:26px">ඉවත් වීම සාර්ථකයි</h1>
<p style="font-size:17px;line-height:1.7">ඔබට තවදුරටත් දිනපතා පුවත් සංග්‍රහය නොලැබේ.</p>
</main></body></html>`))

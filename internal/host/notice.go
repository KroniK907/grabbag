package host

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const (
	noticeCookieName     = "grabbag_notice"
	noticeDefaultSeconds = 3
	noticeMaxSeconds     = 30
)

var noticeTargets = map[string]struct{}{
	"board":    {},
	"host":     {},
	"seated":   {},
	"audience": {},
	"waiting":  {},
}

type noticePayload struct {
	Target   string `json:"target"`
	Type     string `json:"type"`
	Message  string `json:"message"`
	Duration int    `json:"duration"`
}

func clampNoticeSeconds(seconds int) int {
	if seconds < 0 {
		return noticeDefaultSeconds
	}
	if seconds > noticeMaxSeconds {
		return noticeMaxSeconds
	}
	return seconds
}

func (rt *runtime) notify(target, typ, message string, seconds int) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if _, ok := noticeTargets[target]; !ok {
		return ""
	}
	if strings.TrimSpace(typ) == "" {
		typ = "info"
	}
	raw, err := json.Marshal(noticePayload{
		Target:   target,
		Type:     typ,
		Message:  message,
		Duration: clampNoticeSeconds(seconds),
	})
	if err != nil {
		return ""
	}
	line := string(raw)
	rt.events.PublishData("notice", line)
	return line
}

func (rt *runtime) operatorNotice(w http.ResponseWriter, r *http.Request, message string) {
	line := rt.notify("host", "error", message, -1)
	if line != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     noticeCookieName,
			Value:    url.QueryEscape(line),
			Path:     "/",
			MaxAge:   30,
			SameSite: http.SameSiteLaxMode,
		})
	}
	loc := strings.TrimSpace(r.Header.Get("Referer"))
	if loc == "" {
		loc = "/settings"
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

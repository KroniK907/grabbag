package lobby

import (
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/KroniK907/grabbag/internal/ui"
)

func (l *Lobby) stampNotice(r *http.Request, chrome ui.Chrome) ui.Chrome {
	chrome.NoticeTargets = l.noticeTargetList(r)
	return chrome
}

func (l *Lobby) noticeTargets(w http.ResponseWriter, r *http.Request) {
	l.render(w, "notice-targets", ui.Chrome{NoticeTargets: l.noticeTargetList(r)}, http.StatusOK)
}

func (l *Lobby) noticeTargetList(r *http.Request) string {
	path := requestPagePath(r)
	var tags []string
	seen := map[string]struct{}{}
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	if path == "/board" || strings.HasPrefix(path, "/board/") {
		add("board")
	}
	player, ok, err := l.PlayerFromRequest(r)
	if err == nil && ok {
		if player.Seated {
			add("seated")
		} else if player.Waiting {
			add("waiting")
		} else {
			add("audience")
		}
		if player.ClaimedHost && path != "/board" && !strings.HasPrefix(path, "/board/") {
			add("host")
		}
	}
	if l.hasAdminCookie(r) && (path == "/settings" || strings.HasPrefix(path, "/settings/")) {
		add("host")
	}
	sort.Strings(tags)
	return strings.Join(tags, " ")
}

func requestPagePath(r *http.Request) string {
	raw := r.Header.Get("HX-Current-URL")
	if raw == "" {
		raw = r.Referer()
	}
	if raw == "" {
		return r.URL.Path
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}

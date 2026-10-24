package web

import (
	"net/http"
	"strings"
)

// defaultActor is recorded on events when the request carries no trusted
// proxy identity.
const defaultActor = "webui"

// resolveActor returns the actor for r and whether it came from the
// configured identity header. The header is trusted only because ADR 0031
// delegates authentication to a reverse proxy that owns it.
func (s *Server) resolveActor(r *http.Request) (string, bool) {
	if s.cfg.ActorHeader == "" {
		return defaultActor, false
	}
	if v := strings.TrimSpace(r.Header.Get(s.cfg.ActorHeader)); v != "" {
		return v, true
	}
	return defaultActor, false
}

// actorFor returns the actor to record on events produced by r.
func (s *Server) actorFor(r *http.Request) string {
	name, _ := s.resolveActor(r)
	return name
}

// isHeaderToken reports whether name is a valid HTTP field name
// (an RFC 9110 token).
func isHeaderToken(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range []byte(name) {
		if !isTokenChar(c) {
			return false
		}
	}
	return true
}

func isTokenChar(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}

// actorView is the page header's "acting as" label. Source explains where
// Name came from so a header that fails to arrive is visible.
type actorView struct {
	Name   string
	Source string
}

func (s *Server) actorView(r *http.Request) actorView {
	name, fromHeader := s.resolveActor(r)
	header := http.CanonicalHeaderKey(s.cfg.ActorHeader)
	switch {
	case fromHeader:
		return actorView{Name: name, Source: "from header " + header}
	case header == "":
		return actorView{Name: name, Source: "no identity header configured"}
	default:
		return actorView{Name: name, Source: "header " + header + " not received"}
	}
}

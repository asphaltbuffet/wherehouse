package web

import (
	"net/http"
	"net/url"
	"strings"
)

// csrfGuard rejects mutating requests that don't appear to come from the
// browser session served by this process. Two checks combine:
//
//  1. The Hx-Request header must be present. Browsers won't send custom
//     headers on cross-origin form posts without a CORS preflight, which
//     this server does not grant — so the header acts as a same-origin
//     gate that no <form action="..."> attack can satisfy.
//  2. If Origin (or, as a fallback, Referer) is present, its host must
//     match the request's host. This blocks forged requests from other
//     pages the user has open in the same browser.
//
// Safe methods (GET/HEAD/OPTIONS) pass through unchanged.
func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if r.Header.Get("Hx-Request") != htmxHeaderVal {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if !originMatches(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// originMatches returns true when the Origin (or Referer) header's host
// matches the request's Host. If neither header is set the request is
// allowed; the Hx-Request gate is the primary defense and modern browsers
// always set Origin on cross-origin POSTs.
func originMatches(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		raw = r.Header.Get("Referer")
	}
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// securityHeaders sets a baseline set of defensive response headers on
// every response. CSP allows inline scripts/handlers because the
// templates currently embed onclick/hx-on attributes; tighten once those
// move into tree.js.
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// limitBody wraps the request body in an [http.MaxBytesReader] so that
// downstream ParseForm calls reject oversized payloads with 413.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isSafeMethod(r.Method) {
			r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		}
		next.ServeHTTP(w, r)
	})
}

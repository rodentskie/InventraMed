package router

import (
	"net/http"
	"strings"
)

// Router wraps http.ServeMux and applies a path prefix to every route
// registered with Handle. Routes registered with HandleExempt are served
// exactly as written.
type Router struct {
	mux    *http.ServeMux
	prefix string
}

// New returns a Router that prefixes routes with prefix. The prefix must
// already be normalized: a leading "/" and no trailing "/", or empty for none.
func New(prefix string) *Router {
	return &Router{mux: http.NewServeMux(), prefix: prefix}
}

// Handle registers handler for pattern under the prefix, so "POST /login"
// is served at "POST {prefix}/login".
func (r *Router) Handle(pattern string, handler http.HandlerFunc) {
	r.mux.HandleFunc(withPrefix(r.prefix, pattern), handler)
}

// HandleExempt registers handler for pattern without the prefix.
func (r *Router) HandleExempt(pattern string, handler http.HandlerFunc) {
	r.mux.HandleFunc(pattern, handler)
}

// ServeHTTP makes Router an http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// withPrefix puts prefix on the path part of pattern, leaving an optional
// leading method ("GET /login") untouched.
func withPrefix(prefix, pattern string) string {
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		return prefix + pattern
	}

	path = strings.TrimSpace(path)

	return method + " " + prefix + path
}

package httpserver

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(staticFS fs.FS, requestTimeout time.Duration, modules ...func(chi.Router)) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.GetHead)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(requestTimeout))
	router.Use(securityHeaders)

	router.Handle("/static/*", staticHandler(staticFS))

	for _, register := range modules {
		register(router)
	}

	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})

	return router
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// staticHandler serves embedded files and refuses directory requests so the
// embedded file tree is not exposed as an HTML listing.
func staticHandler(staticFS fs.FS) http.Handler {
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Clean(strings.TrimPrefix(r.URL.Path, "/static/"))
		if name == "." || name == "/" {
			http.NotFound(w, r)
			return
		}

		info, err := fs.Stat(staticFS, name)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		fileServer.ServeHTTP(w, r)
	})
}

package processing

import (
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RequestToDiskMw(outPath string, checkDebugEnabledFn func() bool) func(next http.Handler) http.Handler {
	noopMw := func(next http.Handler) http.Handler {
		return next
	}
	if _, err := os.Stat(outPath); err != nil {
		return noopMw
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checkDebugEnabledFn() {
				next.ServeHTTP(w, r)
				return
			}
			fullPath := filepath.Join(outPath, r.Host+strings.ReplaceAll(r.RequestURI, "/", "-")+"-"+time.Now().UTC().Format(time.RFC3339)+".req")
			ff, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			defer r.Body.Close()

			raw, err := httputil.DumpRequest(r, true)
			if err == nil {
				_, _ = ff.Write(raw)
			}

			next.ServeHTTP(w, r)
		})
	}
}

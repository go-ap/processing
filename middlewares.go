package processing

import (
	"bytes"
	"io"
	"net/http"
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
			r2 := cloneRequest(r, ff)
			defer r.Body.Close()

			_, _ = ff.WriteString(r.Method)
			_, _ = ff.WriteString(" ")
			_, _ = ff.WriteString(r.URL.String())
			_, _ = ff.WriteString("\n")
			if len(r.Header) > 0 {
				_ = r.Header.Write(ff)
				_, _ = ff.WriteString("\n\n")
			}
			_, _ = io.ReadAll(r.Body)
			next.ServeHTTP(w, r2)
		})
	}
}

// cloneRequest returns a clone of the provided *http.Request.
func cloneRequest(r *http.Request, ff io.ReadWriter) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r

	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}

	body := bytes.Buffer{}
	// replace old body with the teeReader
	r.Body = io.NopCloser(io.TeeReader(r.Body, io.MultiWriter(ff, &body)))
	// new request body
	r2.Body = io.NopCloser(&body)

	return r2
}

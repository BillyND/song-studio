package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The studio spawns coding agents with broad permissions, so any web page the user
// happens to open must not be able to drive it. Three checks enforce that:
//   - the server binds to loopback only;
//   - the Host header must name this server (blocks DNS rebinding);
//   - every state-changing request carries the per-launch token from the page's
//     <meta> tag and, when sent, a same-origin Origin header (blocks CSRF).

func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func allowedHosts(addr string) map[string]bool {
	_, port, _ := net.SplitHostPort(addr)
	return map[string]bool{
		"127.0.0.1:" + port: true,
		"localhost:" + port: true,
		"[::1]:" + port:     true,
	}
}

func guard(addr, token string, next http.Handler) http.Handler {
	hosts := allowedHosts(addr)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
			http.Error(w, "forbidden cross-site request", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" && !hosts[strings.TrimPrefix(strings.TrimPrefix(origin, "http://"), "https://")] {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			got := r.Header.Get("X-Studio-Token")
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "missing or bad token", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

var errOutside = errors.New("path is outside the allowed directory")

// safeJoin resolves rel inside root and refuses anything that escapes it,
// including through symlinks that already exist on disk.
func safeJoin(root, rel string) (string, error) {
	if rel == "" || strings.ContainsRune(rel, 0) {
		return "", errOutside
	}
	clean := filepath.Clean("/" + filepath.FromSlash(rel))
	full := filepath.Join(root, clean)
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	// Resolve the deepest existing ancestor so new files are checked too.
	probe := full
	for {
		if real, err := filepath.EvalSymlinks(probe); err == nil {
			tail, _ := filepath.Rel(probe, full)
			resolved := filepath.Join(real, tail)
			if resolved != rootReal && !strings.HasPrefix(resolved, rootReal+string(os.PathSeparator)) {
				return "", errOutside
			}
			return full, nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", errOutside
		}
		probe = parent
	}
}

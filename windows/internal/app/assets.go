package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"
)

// assetServer serves the embedded dashboard to WebView2 over loopback HTTP.
//
// WebView2 can only load local content from a real origin (file:// and
// SetVirtualHostNameToFolderMapping both need files on disk), so the simplest
// reliable way to show an embedded FS is a one-line HTTP server bound to
// 127.0.0.1:0. Everything is served under a random path prefix so another local
// process cannot guess the URL, and nothing but the three embedded files is reachable.
type assetServer struct {
	// URL is the absolute URL of index.html.
	URL string

	ln  net.Listener
	srv *http.Server
}

// startAssetServer binds a loopback port and serves fsys from a random prefix.
func startAssetServer(fsys fs.FS) (*assetServer, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("app: listen on loopback: %w", err)
	}

	prefix := "/" + token + "/"
	mux := http.NewServeMux()
	// index.html loads mock.js, which is deliberately not embedded (it is the
	// browser-only fixture bridge). Answer it with an empty script so the popover's
	// console stays clean instead of logging a 404.
	mux.HandleFunc(prefix+"mock.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle(prefix, noStore(http.StripPrefix(prefix, http.FileServerFS(fsys))))

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	return &assetServer{
		URL: "http://" + ln.Addr().String() + prefix + "index.html",
		ln:  ln,
		srv: srv,
	}, nil
}

// Close stops serving.
func (a *assetServer) Close() error {
	if a == nil || a.srv == nil {
		return nil
	}
	return a.srv.Close()
}

// noStore keeps WebView2 from caching a stale dashboard across an app update.
func noStore(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("app: random token: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

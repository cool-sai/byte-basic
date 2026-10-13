package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var resourceKey = regexp.MustCompile(`^[0-9a-f]{32}\.(png|jpg|gif|webp)$`)

func main() {
	listen := getenv("LISTEN", "0.0.0.0:8092")
	origin := strings.TrimRight(getenv("ORIGIN", "http://img.lssl.work"), "/")
	log.Printf("common-static %s origin=%s", listen, origin)
	log.Fatal(http.ListenAndServe(listen, newMux(origin)))
}

func newMux(origin string) http.Handler {
	mux := http.NewServeMux()
	client := &http.Client{Timeout: 20 * time.Second}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("GET /img/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !resourceKey.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		pull(w, r, client, origin+"/img/"+name)
	})
	return mux
}

func pull(w http.ResponseWriter, r *http.Request, client *http.Client, rawURL string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	res, err := client.Do(req)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		status := res.StatusCode
		if status >= 500 {
			status = http.StatusBadGateway
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	media := strings.TrimSpace(strings.Split(res.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(media, "image/") {
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", media)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, res.Body)
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

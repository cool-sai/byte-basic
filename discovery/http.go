package discovery

import (
	"encoding/json"
	"net/http"
	"time"
)

type Instance struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
}

func Handler(reg *Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var in Instance
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" || in.Addr == "" {
			http.Error(w, "need {name,addr}", http.StatusBadRequest)
			return
		}
		reg.Register(in.Name, in.Addr, time.Now())
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/lookup", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		addrs := reg.Lookup(name, time.Now())
		if addrs == nil {
			addrs = []string{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]string{"addrs": addrs})
	})
	return mux
}

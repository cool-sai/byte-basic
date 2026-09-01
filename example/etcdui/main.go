package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"

	"minikitex/config"
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	base := getenv("ETCD", "http://127.0.0.1:2379")
	addr := getenv("LISTEN", "127.0.0.1:2381")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		_ = page.Execute(w, nil)
	})
	mux.HandleFunc("/api/kvs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			kvs, err := config.List(base)
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			if kvs == nil {
				kvs = []config.KV{}
			}
			_ = json.NewEncoder(w).Encode(kvs)
		case http.MethodPut:
			var kv config.KV
			if err := json.NewDecoder(r.Body).Decode(&kv); err != nil || kv.Key == "" {
				http.Error(w, "need {key,value}", 400)
				return
			}
			if err := config.Put(base, kv.Key, kv.Value); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			key := r.URL.Query().Get("key")
			if key == "" {
				http.Error(w, "key", 400)
				return
			}
			if err := config.Delete(base, key); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method", 405)
		}
	})
	log.Println("etcd ui", addr, "->", base)
	log.Fatal(http.ListenAndServe(addr, mux))
}

var page = template.Must(template.New("p").Parse(`<!doctype html>
<meta charset="utf-8">
<title>etcd</title>
<style>
  body { font: 14px/1.4 -apple-system, sans-serif; max-width: 720px; margin: 32px auto; color: #222; }
  h1 { font-size: 18px; }
  table { border-collapse: collapse; width: 100%; margin: 16px 0; }
  th, td { border-bottom: 1px solid #ddd; padding: 6px 8px; text-align: left; }
  input { font: inherit; padding: 4px 8px; }
  input[name=key] { width: 220px; }
  input[name=value] { width: 220px; }
  button { font: inherit; padding: 4px 10px; cursor: pointer; }
  .err { color: #b00020; min-height: 1.4em; }
</style>
<h1>etcd</h1>
<p class="err" id="err"></p>
<table>
  <thead><tr><th>key</th><th>value</th><th></th></tr></thead>
  <tbody id="rows"></tbody>
</table>
<form id="f">
  <input name="key" placeholder="user/name_suffix" required>
  <input name="value" placeholder="!!!">
  <button>保存</button>
</form>
<script>
const err = document.getElementById('err')
const rows = document.getElementById('rows')
async function load() {
  err.textContent = ''
  const r = await fetch('/api/kvs')
  if (!r.ok) { err.textContent = await r.text(); return }
  const kvs = await r.json()
  rows.innerHTML = kvs.map(kv =>
    '<tr><td><code>'+esc(kv.Key)+'</code></td><td>'+esc(kv.Value)+'</td><td><button data-key="'+esc(kv.Key)+'">删除</button></td></tr>'
  ).join('')
}
function esc(s){return String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
document.getElementById('f').onsubmit = async (e) => {
  e.preventDefault()
  const fd = new FormData(e.target)
  const r = await fetch('/api/kvs', {method:'PUT', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({Key: fd.get('key'), Value: fd.get('value')})})
  if (!r.ok) { err.textContent = await r.text(); return }
  e.target.reset(); load()
}
rows.onclick = async (e) => {
  const b = e.target.closest('button')
  if (!b) return
  const r = await fetch('/api/kvs?key='+encodeURIComponent(b.dataset.key), {method:'DELETE'})
  if (!r.ok) { err.textContent = await r.text(); return }
  load()
}
load()
</script>
`))

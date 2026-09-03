package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type userKey struct{}

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "minikitex-jwt"
	}
	return []byte(s)
}

func seedAdmin(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS account (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(64) NOT NULL UNIQUE,
			pass_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return err
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM account`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("minikitex"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO account (name, pass_hash) VALUES (?,?)`, "admin", hash)
	return err
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		fail(w, 400, err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || body.Password == "" {
		fail(w, 400, fmt.Errorf("请输入用户名和密码"))
		return
	}
	var hash string
	err := s.db.QueryRow(`SELECT pass_hash FROM account WHERE name=?`, body.Name).Scan(&hash)
	if err == sql.ErrNoRows {
		fail(w, 401, fmt.Errorf("用户名或密码错误"))
		return
	}
	if err != nil {
		fail(w, 500, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		fail(w, 401, fmt.Errorf("用户名或密码错误"))
		return
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   body.Name,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})
	signed, err := tok.SignedString(jwtSecret())
	if err != nil {
		fail(w, 500, err)
		return
	}
	writeJSON(w, map[string]any{"token": signed, "name": body.Name})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	name, _ := r.Context().Value(userKey{}).(string)
	writeJSON(w, map[string]any{"name": name})
}

func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login" {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if raw == "" || raw == r.Header.Get("Authorization") {
			fail(w, 401, fmt.Errorf("未登录"))
			return
		}
		claims := &jwt.RegisteredClaims{}
		tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("alg")
			}
			return jwtSecret(), nil
		})
		if err != nil || !tok.Valid || claims.Subject == "" {
			fail(w, 401, fmt.Errorf("未登录"))
			return
		}
		ctx := context.WithValue(r.Context(), userKey{}, claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	v1 "minikitex/gen/platform/v1"
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

func parseToken(raw string) (string, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	if raw == "" {
		return "", unauth(fmt.Errorf("未登录"))
	}
	claims := &jwt.RegisteredClaims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("alg")
		}
		return jwtSecret(), nil
	})
	if err != nil || !tok.Valid || claims.Subject == "" {
		return "", unauth(fmt.Errorf("未登录"))
	}
	return claims.Subject, nil
}

func (s *server) Login(_ context.Context, req *connect.Request[v1.LoginRequest]) (*connect.Response[v1.LoginResponse], error) {
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" || req.Msg.Password == "" {
		return nil, invalid(fmt.Errorf("请输入用户名和密码"))
	}
	var hash string
	err := s.db.QueryRow(`SELECT pass_hash FROM account WHERE name=?`, name).Scan(&hash)
	if err == sql.ErrNoRows {
		return nil, unauth(fmt.Errorf("用户名或密码错误"))
	}
	if err != nil {
		return nil, internal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Msg.Password)) != nil {
		return nil, unauth(fmt.Errorf("用户名或密码错误"))
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   name,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})
	signed, err := tok.SignedString(jwtSecret())
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.LoginResponse{Token: signed, Name: name}), nil
}

func (s *server) Me(ctx context.Context, _ *connect.Request[v1.MeRequest]) (*connect.Response[v1.MeResponse], error) {
	name, _ := ctx.Value(userKey{}).(string)
	return connect.NewResponse(&v1.MeResponse{Name: name}), nil
}

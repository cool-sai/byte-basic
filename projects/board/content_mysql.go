package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// mysqlContent 与旧的 board_kind/board_item 共用一个库，但只读写自己的三张新表。
type mysqlContent struct {
	db *sql.DB
}

func newMySQLContent(db *sql.DB) (*mysqlContent, error) {
	c := &mysqlContent{db: db}
	if err := c.migrate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *mysqlContent) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS board_node (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			parent_id BIGINT NOT NULL DEFAULT 0,
			type VARCHAR(16) NOT NULL,
			slug VARCHAR(128) NOT NULL,
			title VARCHAR(255) NOT NULL DEFAULT '',
			data JSON,
			schema_version INT NOT NULL DEFAULT 1,
			deleted_at TIMESTAMP NULL DEFAULT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_parent_slug (parent_id, slug),
			INDEX idx_parent_type_created (parent_id, type, created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS board_page (
			path VARCHAR(255) PRIMARY KEY,
			current_version INT NOT NULL DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS board_page_version (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			path VARCHAR(255) NOT NULL,
			version INT NOT NULL,
			files LONGTEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_path_version (path, version)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate content: %w", err)
		}
	}
	return nil
}

const nodeCols = `id, parent_id, type, slug, title, data, schema_version, created_at, updated_at`

func scanNode(sc scanner) (Node, error) {
	var n Node
	var data []byte
	var created, updated time.Time
	if err := sc.Scan(&n.ID, &n.ParentID, &n.Type, &n.Slug, &n.Title, &data, &n.SchemaVersion, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, errNotFound
		}
		return Node{}, err
	}
	if len(data) == 0 {
		data = []byte(`{}`)
	}
	n.Data = json.RawMessage(data)
	n.CreatedAt = fmtTime(created)
	n.UpdatedAt = fmtTime(updated)
	return n, nil
}

type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}

func (s *mysqlContent) liveNode(q queryer, id int64, lock bool) (Node, error) {
	sfx := ""
	if lock {
		sfx = " FOR UPDATE"
	}
	return scanNode(q.QueryRow(`SELECT `+nodeCols+` FROM board_node WHERE id=? AND deleted_at IS NULL`+sfx, id))
}

func (s *mysqlContent) PutNode(in NodeInput) (Node, error) {
	slug, err := cleanSlug(in.Slug)
	if err != nil {
		return Node{}, err
	}
	data, err := cleanNodeData(in.Data)
	if err != nil {
		return Node{}, err
	}
	sv, err := cleanSchemaVersion(in.SchemaVersion)
	if err != nil {
		return Node{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Node{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var parent *Node
	if in.ParentID != 0 {
		p, err := s.liveNode(tx, in.ParentID, true)
		if err != nil {
			return Node{}, err
		}
		parent = &p
	}
	if err := checkParent(in.Type, parent); err != nil {
		return Node{}, err
	}
	var id int64
	err = tx.QueryRow(`SELECT id FROM board_node WHERE parent_id=? AND slug=? FOR UPDATE`, in.ParentID, slug).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.Exec(`INSERT INTO board_node (parent_id, type, slug, title, data, schema_version) VALUES (?,?,?,?,?,?)`,
			in.ParentID, in.Type, slug, in.Title, string(data), sv)
		if err != nil {
			return Node{}, err
		}
		id, _ = res.LastInsertId()
	case err != nil:
		return Node{}, err
	default:
		// 同 parent+slug：覆盖；软删除过的复活。
		if _, err := tx.Exec(`UPDATE board_node SET type=?, title=?, data=?, schema_version=?, deleted_at=NULL WHERE id=?`,
			in.Type, in.Title, string(data), sv, id); err != nil {
			return Node{}, err
		}
	}
	n, err := s.liveNode(tx, id, false)
	if err != nil {
		return Node{}, err
	}
	return n, tx.Commit()
}

func (s *mysqlContent) UpdateNode(id int64, title *string, data *json.RawMessage, schemaVersion *int) (Node, error) {
	sets := []string{}
	args := []any{}
	if title != nil {
		sets = append(sets, "title=?")
		args = append(args, *title)
	}
	if data != nil {
		d, err := cleanNodeData(*data)
		if err != nil {
			return Node{}, err
		}
		sets = append(sets, "data=?")
		args = append(args, string(d))
	}
	if schemaVersion != nil {
		sv, err := cleanSchemaVersion(*schemaVersion)
		if err != nil {
			return Node{}, err
		}
		sets = append(sets, "schema_version=?")
		args = append(args, sv)
	}
	if _, err := s.liveNode(s.db, id, false); err != nil {
		return Node{}, err
	}
	if len(sets) > 0 {
		args = append(args, id)
		if _, err := s.db.Exec(`UPDATE board_node SET `+strings.Join(sets, ", ")+` WHERE id=? AND deleted_at IS NULL`, args...); err != nil {
			return Node{}, err
		}
	}
	return s.liveNode(s.db, id, false)
}

// DeleteNode 软删除节点及整棵子树，返回删除的节点数。
func (s *mysqlContent) DeleteNode(id int64) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := s.liveNode(tx, id, true); err != nil {
		return 0, err
	}
	all := []int64{id}
	frontier := []int64{id}
	for len(frontier) > 0 && len(all) < 100000 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(frontier)), ",")
		args := make([]any, len(frontier))
		for i, v := range frontier {
			args[i] = v
		}
		rows, err := tx.Query(`SELECT id FROM board_node WHERE deleted_at IS NULL AND parent_id IN (`+ph+`)`, args...)
		if err != nil {
			return 0, err
		}
		next := []int64{}
		for rows.Next() {
			var c int64
			if err := rows.Scan(&c); err != nil {
				rows.Close()
				return 0, err
			}
			next = append(next, c)
		}
		rows.Close()
		all = append(all, next...)
		frontier = next
	}
	for i := 0; i < len(all); i += 500 {
		chunk := all[i:min(i+500, len(all))]
		ph := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		args := make([]any, len(chunk))
		for j, v := range chunk {
			args[j] = v
		}
		if _, err := tx.Exec(`UPDATE board_node SET deleted_at=NOW() WHERE deleted_at IS NULL AND id IN (`+ph+`)`, args...); err != nil {
			return 0, err
		}
	}
	return len(all), tx.Commit()
}

func (s *mysqlContent) GetNode(id int64) (Node, error) { return s.liveNode(s.db, id, false) }

func (s *mysqlContent) FindChild(parentID int64, typ, slug string) (Node, error) {
	return scanNode(s.db.QueryRow(`SELECT `+nodeCols+` FROM board_node WHERE parent_id=? AND type=? AND slug=? AND deleted_at IS NULL`, parentID, typ, slug))
}

func (s *mysqlContent) ListChildren(parentID int64, typ string, page, size int) (NodePage, error) {
	page, size = nodePageArgs(page, size)
	out := NodePage{Items: []Node{}, Page: page, PageSize: size}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM board_node WHERE parent_id=? AND type=? AND deleted_at IS NULL`, parentID, typ).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.db.Query(`SELECT `+nodeCols+` FROM board_node WHERE parent_id=? AND type=? AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, parentID, typ, size, (page-1)*size)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, n)
	}
	return out, rows.Err()
}

func (s *mysqlContent) PutPage(p string, files map[string]string) (PageVersion, error) {
	p, err := cleanPagePath(p)
	if err != nil {
		return PageVersion{}, err
	}
	files, err = cleanPageFiles(files)
	if err != nil {
		return PageVersion{}, err
	}
	raw, err := json.Marshal(files)
	if err != nil {
		return PageVersion{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PageVersion{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`INSERT INTO board_page (path, current_version) VALUES (?, 0) ON DUPLICATE KEY UPDATE path=path`, p); err != nil {
		return PageVersion{}, err
	}
	var cur int
	if err := tx.QueryRow(`SELECT current_version FROM board_page WHERE path=? FOR UPDATE`, p).Scan(&cur); err != nil {
		return PageVersion{}, err
	}
	var maxV int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version),0) FROM board_page_version WHERE path=?`, p).Scan(&maxV); err != nil {
		return PageVersion{}, err
	}
	v := maxV + 1
	if _, err := tx.Exec(`INSERT INTO board_page_version (path, version, files) VALUES (?,?,?)`, p, v, string(raw)); err != nil {
		return PageVersion{}, err
	}
	if _, err := tx.Exec(`UPDATE board_page SET current_version=? WHERE path=?`, v, p); err != nil {
		return PageVersion{}, err
	}
	if err := tx.Commit(); err != nil {
		return PageVersion{}, err
	}
	return PageVersion{Path: p, Version: v, Files: sortedKeys(files), Current: true, CreatedAt: nowStr()}, nil
}

func (s *mysqlContent) ListPageVersions(p string) ([]PageVersion, error) {
	p, err := cleanPagePath(p)
	if err != nil {
		return nil, err
	}
	var cur int
	if err := s.db.QueryRow(`SELECT current_version FROM board_page WHERE path=?`, p).Scan(&cur); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}
	rows, err := s.db.Query(`SELECT version, files, created_at FROM board_page_version WHERE path=? ORDER BY version DESC LIMIT 200`, p)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PageVersion{}
	for rows.Next() {
		var v int
		var raw string
		var at time.Time
		if err := rows.Scan(&v, &raw, &at); err != nil {
			return nil, err
		}
		var files map[string]string
		_ = json.Unmarshal([]byte(raw), &files)
		out = append(out, PageVersion{Path: p, Version: v, Files: sortedKeys(files), Current: v == cur, CreatedAt: fmtTime(at)})
	}
	return out, rows.Err()
}

func (s *mysqlContent) RollbackPage(p string, version int) (PageVersion, error) {
	p, err := cleanPagePath(p)
	if err != nil {
		return PageVersion{}, err
	}
	var raw string
	var at time.Time
	if err := s.db.QueryRow(`SELECT files, created_at FROM board_page_version WHERE path=? AND version=?`, p, version).Scan(&raw, &at); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PageVersion{}, errNotFound
		}
		return PageVersion{}, err
	}
	if _, err := s.db.Exec(`UPDATE board_page SET current_version=? WHERE path=?`, version, p); err != nil {
		return PageVersion{}, err
	}
	var files map[string]string
	_ = json.Unmarshal([]byte(raw), &files)
	return PageVersion{Path: p, Version: version, Files: sortedKeys(files), Current: true, CreatedAt: fmtTime(at)}, nil
}

func (s *mysqlContent) PageFiles(p string) (map[string]string, error) {
	var raw string
	err := s.db.QueryRow(`SELECT v.files FROM board_page pg JOIN board_page_version v ON v.path=pg.path AND v.version=pg.current_version WHERE pg.path=?`, p).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	var files map[string]string
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil, err
	}
	return files, nil
}

// fmtTime：驱动按 UTC 解析 TIMESTAMP（DSN 没带 loc），展示统一转成进程时区（容器 TZ=Asia/Shanghai）。
func fmtTime(t time.Time) string { return t.In(time.Local).Format("2006-01-02 15:04:05") }

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type mysqlBoard struct {
	db *sql.DB
}

func openMySQL(dsn string) (*mysqlBoard, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(10)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	b := &mysqlBoard{db: db}
	if err := b.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return b, nil
}

func (s *mysqlBoard) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS board_kind (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(64) NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS board_item (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			kind_id BIGINT NOT NULL,
			content TEXT NOT NULL,
			type VARCHAR(64) NOT NULL DEFAULT '',
			item_date DATE NOT NULL,
			extra JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_kind_date (kind_id, item_date),
			CONSTRAINT fk_board_item_kind FOREIGN KEY (kind_id) REFERENCES board_kind(id)
		)`,
		`CREATE TABLE IF NOT EXISTS board_kind_html (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			kind_id BIGINT NOT NULL UNIQUE,
			html TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			CONSTRAINT fk_board_kind_html_kind FOREIGN KEY (kind_id) REFERENCES board_kind(id)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *mysqlBoard) CreateKind(name string) (Kind, error) {
	name, err := cleanName(name)
	if err != nil {
		return Kind{}, err
	}
	res, err := s.db.Exec(`INSERT INTO board_kind (name) VALUES (?)`, name)
	if err != nil {
		if isDup(err) {
			return s.kindByName(name)
		}
		return Kind{}, err
	}
	id, _ := res.LastInsertId()
	return Kind{ID: id, Name: name}, nil
}

func (s *mysqlBoard) kindByName(name string) (Kind, error) {
	var k Kind
	err := s.db.QueryRow(`SELECT id, name FROM board_kind WHERE name=?`, name).Scan(&k.ID, &k.Name)
	if err == sql.ErrNoRows {
		return Kind{}, fmt.Errorf("没有种类 %s", name)
	}
	return k, err
}

func (s *mysqlBoard) AddItem(kind, content, date, typ string, extra json.RawMessage) (Item, error) {
	k, err := s.kindByName(strings.TrimSpace(kind))
	if err != nil {
		return Item{}, err
	}
	content, err = cleanContent(content)
	if err != nil {
		return Item{}, err
	}
	day, err := parseDay(date)
	if err != nil {
		return Item{}, err
	}
	typ = strings.TrimSpace(typ)
	if len([]rune(typ)) > 64 {
		return Item{}, fmt.Errorf("type 太长")
	}
	ex := extraOrEmpty(extra)
	res, err := s.db.Exec(
		`INSERT INTO board_item (kind_id, content, type, item_date, extra) VALUES (?,?,?,?,?)`,
		k.ID, content, typ, day, []byte(ex),
	)
	if err != nil {
		return Item{}, err
	}
	id, _ := res.LastInsertId()
	return s.itemByID(id)
}

func (s *mysqlBoard) UpdateItem(id int64, kind, content, date, typ *string, extra *json.RawMessage) (Item, error) {
	cur, err := s.itemByID(id)
	if err != nil {
		return Item{}, err
	}
	kindID := cur.KindID
	if kind != nil {
		k, err := s.kindByName(strings.TrimSpace(*kind))
		if err != nil {
			return Item{}, err
		}
		kindID = k.ID
	}
	body := cur.Content
	if content != nil {
		body, err = cleanContent(*content)
		if err != nil {
			return Item{}, err
		}
	}
	day := cur.Date
	if date != nil {
		day, err = parseDay(*date)
		if err != nil {
			return Item{}, err
		}
	}
	itemType := cur.Type
	if typ != nil {
		itemType = strings.TrimSpace(*typ)
		if len([]rune(itemType)) > 64 {
			return Item{}, fmt.Errorf("type 太长")
		}
	}
	ex := extraOrEmpty(mapExtra(cur.Extra))
	if extra != nil {
		ex = extraOrEmpty(*extra)
	}
	_, err = s.db.Exec(
		`UPDATE board_item SET kind_id=?, content=?, type=?, item_date=?, extra=? WHERE id=?`,
		kindID, body, itemType, day, []byte(ex), id,
	)
	if err != nil {
		return Item{}, err
	}
	return s.itemByID(id)
}

func (s *mysqlBoard) DeleteItem(id int64) error {
	res, err := s.db.Exec(`DELETE FROM board_item WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return os.ErrNotExist
	}
	return nil
}

func (s *mysqlBoard) ListItems(kind string, page, pageSize int) (ListPage, error) {
	k, err := s.kindByName(strings.TrimSpace(kind))
	if err != nil {
		return ListPage{}, err
	}
	page, pageSize = pageArgs(page, pageSize)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM board_item WHERE kind_id=?`, k.ID).Scan(&total); err != nil {
		return ListPage{}, err
	}
	rows, err := s.db.Query(
		`SELECT i.id, i.kind_id, k.name, i.content, i.type, i.item_date, i.extra
		 FROM board_item i JOIN board_kind k ON k.id=i.kind_id
		 WHERE i.kind_id=?
		 ORDER BY i.item_date DESC, i.id DESC
		 LIMIT ? OFFSET ?`,
		k.ID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		return ListPage{}, err
	}
	defer rows.Close()
	items, err := scanItems(rows)
	if err != nil {
		return ListPage{}, err
	}
	if h, err := s.htmlByKindID(k.ID); err == nil {
		applyHTML(items, h.HTML)
	}
	return ListPage{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *mysqlBoard) CreateKindHTML(kind, html string) (KindHTML, error) {
	k, err := s.kindByName(strings.TrimSpace(kind))
	if err != nil {
		return KindHTML{}, err
	}
	html, err = parseKindHTML(html)
	if err != nil {
		return KindHTML{}, err
	}
	res, err := s.db.Exec(`INSERT INTO board_kind_html (kind_id, html) VALUES (?,?)`, k.ID, html)
	if err != nil {
		if isDup(err) {
			return KindHTML{}, fmt.Errorf("这个种类已经有 html 片段")
		}
		return KindHTML{}, err
	}
	id, _ := res.LastInsertId()
	return s.htmlByID(id)
}

func (s *mysqlBoard) UpdateKindHTML(kind, html string) (KindHTML, error) {
	cur, err := s.GetKindHTML(kind)
	if err != nil {
		return KindHTML{}, err
	}
	html, err = parseKindHTML(html)
	if err != nil {
		return KindHTML{}, err
	}
	if _, err := s.db.Exec(`UPDATE board_kind_html SET html=? WHERE id=?`, html, cur.ID); err != nil {
		return KindHTML{}, err
	}
	return s.htmlByID(cur.ID)
}

func (s *mysqlBoard) DeleteKindHTML(kind string) error {
	cur, err := s.GetKindHTML(kind)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM board_kind_html WHERE id=?`, cur.ID)
	return err
}

func (s *mysqlBoard) GetKindHTML(kind string) (KindHTML, error) {
	k, err := s.kindByName(strings.TrimSpace(kind))
	if err != nil {
		return KindHTML{}, err
	}
	h, err := s.htmlByKindID(k.ID)
	if err == sql.ErrNoRows {
		return KindHTML{}, fmt.Errorf("这个种类还没有 html 片段")
	}
	return h, err
}

func (s *mysqlBoard) htmlByKindID(kindID int64) (KindHTML, error) {
	row := s.db.QueryRow(
		`SELECT h.id, h.kind_id, k.name, h.html
		 FROM board_kind_html h JOIN board_kind k ON k.id=h.kind_id
		 WHERE h.kind_id=?`, kindID,
	)
	return scanKindHTML(row)
}

func (s *mysqlBoard) htmlByID(id int64) (KindHTML, error) {
	row := s.db.QueryRow(
		`SELECT h.id, h.kind_id, k.name, h.html
		 FROM board_kind_html h JOIN board_kind k ON k.id=h.kind_id
		 WHERE h.id=?`, id,
	)
	h, err := scanKindHTML(row)
	if err == sql.ErrNoRows {
		return KindHTML{}, os.ErrNotExist
	}
	return h, err
}

func scanKindHTML(sc scanner) (KindHTML, error) {
	var h KindHTML
	if err := sc.Scan(&h.ID, &h.KindID, &h.Kind, &h.HTML); err != nil {
		return KindHTML{}, err
	}
	return h, nil
}

func (s *mysqlBoard) Summary() (Summary, error) {
	rows, err := s.db.Query(
		`SELECT k.id, k.name, COUNT(i.id)
		 FROM board_kind k
		 LEFT JOIN board_item i ON i.kind_id=k.id
		 GROUP BY k.id, k.name
		 ORDER BY k.id`,
	)
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()
	out := Summary{Kinds: []KindCount{}}
	for rows.Next() {
		var c KindCount
		if err := rows.Scan(&c.ID, &c.Name, &c.Count); err != nil {
			return Summary{}, err
		}
		out.Kinds = append(out.Kinds, c)
		out.Total += c.Count
	}
	if err := rows.Err(); err != nil {
		return Summary{}, err
	}
	var t sql.NullTime
	if err := s.db.QueryRow(`SELECT MAX(updated_at) FROM board_item`).Scan(&t); err != nil {
		return Summary{}, err
	}
	if t.Valid {
		out.UpdatedAt = t.Time.In(time.Local).Format(time.RFC3339)
	}
	return out, nil
}

func (s *mysqlBoard) itemByID(id int64) (Item, error) {
	row := s.db.QueryRow(
		`SELECT i.id, i.kind_id, k.name, i.content, i.type, i.item_date, i.extra
		 FROM board_item i JOIN board_kind k ON k.id=i.kind_id
		 WHERE i.id=?`, id,
	)
	it, err := scanItem(row)
	if err == sql.ErrNoRows {
		return Item{}, os.ErrNotExist
	}
	return it, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(sc scanner) (Item, error) {
	var it Item
	var day time.Time
	var extra []byte
	if err := sc.Scan(&it.ID, &it.KindID, &it.Kind, &it.Content, &it.Type, &day, &extra); err != nil {
		return Item{}, err
	}
	it.Date = day.Format("2006-01-02")
	it.Extra = extraMap(extra)
	return it, nil
}

func scanItems(rows *sql.Rows) ([]Item, error) {
	items := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func isDup(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

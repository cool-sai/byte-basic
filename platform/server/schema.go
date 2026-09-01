package main

import (
	"fmt"
	"net/http"
	"regexp"
)

var tableName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func (s *server) listTables(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`
		SELECT TABLE_NAME, TABLE_ROWS, ENGINE, CREATE_TIME, UPDATE_TIME, TABLE_COMMENT
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer rows.Close()
	writeJSON(w, scanRows(rows, "name", "rows", "engine", "createdAt", "updatedAt", "comment"))
}

func (s *server) getTable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !tableName.MatchString(name) {
		fail(w, 400, fmt.Errorf("bad table name"))
		return
	}
	cols, err := s.db.Query(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, name)
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer cols.Close()
	columns := scanRows(cols, "name", "type", "nullable", "key", "default", "extra", "comment")

	data, err := s.db.Query("SELECT * FROM `" + name + "` LIMIT 20")
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer data.Close()
	fields, _ := data.Columns()
	var preview []map[string]any
	for data.Next() {
		vals := make([]any, len(fields))
		ptrs := make([]any, len(fields))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := data.Scan(ptrs...); err != nil {
			fail(w, 500, err)
			return
		}
		row := map[string]any{}
		for i, f := range fields {
			row[f] = stringify(vals[i])
		}
		preview = append(preview, row)
	}
	writeJSON(w, map[string]any{"name": name, "columns": columns, "preview": preview})
}

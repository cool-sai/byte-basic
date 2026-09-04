package main

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"connectrpc.com/connect"

	v1 "minikitex/gen/platform/v1"
)

var tableName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func (s *server) ListTables(context.Context, *connect.Request[v1.ListTablesRequest]) (*connect.Response[v1.ListTablesResponse], error) {
	rows, err := s.db.Query(`
		SELECT TABLE_NAME, TABLE_ROWS, ENGINE, CREATE_TIME, UPDATE_TIME, TABLE_COMMENT
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`)
	if err != nil {
		return nil, internal(err)
	}
	defer rows.Close()
	var tables []*v1.DbTable
	for rows.Next() {
		t := &v1.DbTable{}
		var nrows sql.NullInt64
		var created, updated sql.NullTime
		if err := rows.Scan(&t.Name, &nrows, &t.Engine, &created, &updated, &t.Comment); err != nil {
			return nil, internal(err)
		}
		if nrows.Valid {
			t.Rows = fmt.Sprint(nrows.Int64)
		}
		if created.Valid {
			t.CreatedAt = fmtTime(created.Time)
		}
		if updated.Valid {
			t.UpdatedAt = fmtTime(updated.Time)
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.ListTablesResponse{Tables: tables}), nil
}

func (s *server) GetTable(_ context.Context, req *connect.Request[v1.GetTableRequest]) (*connect.Response[v1.Table], error) {
	name := req.Msg.Name
	if !tableName.MatchString(name) {
		return nil, invalid(fmt.Errorf("bad table name"))
	}
	cols, err := s.db.Query(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, name)
	if err != nil {
		return nil, internal(err)
	}
	defer cols.Close()
	var columns []*v1.DbColumn
	for cols.Next() {
		c := &v1.DbColumn{}
		var def sql.NullString
		if err := cols.Scan(&c.Name, &c.Type, &c.Nullable, &c.Key, &def, &c.Extra, &c.Comment); err != nil {
			return nil, internal(err)
		}
		if def.Valid {
			c.DefaultValue = def.String
		}
		columns = append(columns, c)
	}
	if err := cols.Err(); err != nil {
		return nil, internal(err)
	}

	data, err := s.db.Query("SELECT * FROM `" + name + "` LIMIT 20")
	if err != nil {
		return nil, internal(err)
	}
	defer data.Close()
	fields, _ := data.Columns()
	var preview []*v1.TableRow
	for data.Next() {
		vals := make([]any, len(fields))
		ptrs := make([]any, len(fields))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := data.Scan(ptrs...); err != nil {
			return nil, internal(err)
		}
		cells := map[string]string{}
		for i, f := range fields {
			cells[f] = cellString(vals[i])
		}
		preview = append(preview, &v1.TableRow{Cells: cells})
	}
	return connect.NewResponse(&v1.Table{Name: name, Columns: columns, Preview: preview}), nil
}

func cellString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case time.Time:
		return fmtTime(x)
	default:
		return fmt.Sprint(x)
	}
}

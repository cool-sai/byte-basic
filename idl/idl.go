package idl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type Spec struct {
	Service string
	Methods []Method
	Structs map[string]*Struct
}

type Method struct {
	Name string
	Req  string
	Resp string
}

type Struct struct {
	Name   string
	Fields []Field
}

type Field struct {
	ID   int
	Type string // i64 | string | bool
	Name string
}

func ParseFile(path string) (*Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

func ParseString(s string) (*Spec, error) {
	return Parse(strings.NewReader(s))
}

func Parse(r io.Reader) (*Spec, error) {
	spec := &Spec{Structs: map[string]*Struct{}}
	sc := bufio.NewScanner(r)

	const (
		stTop = iota
		stService
		stStruct
	)
	state := stTop
	var cur *Struct

	for sc.Scan() {
		line := stripComment(sc.Text())
		if line == "" || line == "{" {
			continue
		}
		if line == "}" {
			state = stTop
			cur = nil
			continue
		}

		switch state {
		case stTop:
			switch {
			case strings.HasPrefix(line, "service "):
				spec.Service = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "service "), "{"))
				if spec.Service == "" {
					return nil, fmt.Errorf("idl: empty service name")
				}
				state = stService
			case strings.HasPrefix(line, "struct "):
				name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "struct "), "{"))
				if name == "" {
					return nil, fmt.Errorf("idl: empty struct name")
				}
				cur = &Struct{Name: name}
				spec.Structs[name] = cur
				state = stStruct
			default:
				return nil, fmt.Errorf("idl: unexpected %q", line)
			}
		case stService:
			name, req, resp, err := parseMethod(line)
			if err != nil {
				return nil, err
			}
			spec.Methods = append(spec.Methods, Method{Name: name, Req: req, Resp: resp})
		case stStruct:
			f, err := parseField(line)
			if err != nil {
				return nil, err
			}
			cur.Fields = append(cur.Fields, f)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return spec, spec.validate()
}

func (s *Spec) Method(name string) (Method, bool) {
	for _, m := range s.Methods {
		if m.Name == name {
			return m, true
		}
	}
	return Method{}, false
}

func (s *Spec) Struct(name string) (*Struct, error) {
	st, ok := s.Structs[name]
	if !ok {
		return nil, fmt.Errorf("idl: unknown struct %s", name)
	}
	return st, nil
}

func (st *Struct) FieldByID(id int) (Field, bool) {
	for _, f := range st.Fields {
		if f.ID == id {
			return f, true
		}
	}
	return Field{}, false
}

func (st *Struct) FieldByName(name string) (Field, bool) {
	for _, f := range st.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

func (s *Spec) validate() error {
	if s.Service == "" {
		return fmt.Errorf("idl: missing service")
	}
	if len(s.Methods) == 0 {
		return fmt.Errorf("idl: no methods")
	}
	for _, m := range s.Methods {
		if _, err := s.Struct(m.Req); err != nil {
			return fmt.Errorf("idl: method %s req: %w", m.Name, err)
		}
		if _, err := s.Struct(m.Resp); err != nil {
			return fmt.Errorf("idl: method %s resp: %w", m.Name, err)
		}
	}
	for _, st := range s.Structs {
		seen := map[int]string{}
		for _, f := range st.Fields {
			if prev, ok := seen[f.ID]; ok {
				return fmt.Errorf("idl: %s field id %d reused by %s and %s", st.Name, f.ID, prev, f.Name)
			}
			seen[f.ID] = f.Name
		}
	}
	return nil
}

func GoName(name string) string {
	if name == "id" {
		return "ID"
	}
	if name == "" {
		return ""
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func GoType(t string) string {
	switch t {
	case "i64":
		return "int64"
	case "string":
		return "string"
	case "bool":
		return "bool"
	default:
		return t
	}
}

func stripComment(s string) string {
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "//"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func parseMethod(line string) (name, req, resp string, err error) {
	line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
	i := strings.IndexByte(line, '(')
	j := strings.IndexByte(line, ')')
	if i < 1 || j < i {
		return "", "", "", fmt.Errorf("idl: bad method %q", line)
	}
	name = strings.TrimSpace(line[:i])
	req = strings.TrimSpace(line[i+1 : j])
	resp = strings.TrimSpace(line[j+1:])
	if name == "" || req == "" || resp == "" {
		return "", "", "", fmt.Errorf("idl: bad method %q", line)
	}
	return name, req, resp, nil
}

func parseField(line string) (Field, error) {
	line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
	colon := strings.IndexByte(line, ':')
	if colon < 1 {
		return Field{}, fmt.Errorf("idl: bad field %q", line)
	}
	id, err := strconv.Atoi(strings.TrimSpace(line[:colon]))
	if err != nil {
		return Field{}, fmt.Errorf("idl: bad field id in %q", line)
	}
	rest := strings.Fields(strings.TrimSpace(line[colon+1:]))
	if len(rest) != 2 {
		return Field{}, fmt.Errorf("idl: bad field %q", line)
	}
	if rest[0] != "i64" && rest[0] != "string" && rest[0] != "bool" {
		return Field{}, fmt.Errorf("idl: unsupported type %s", rest[0])
	}
	return Field{ID: id, Type: rest[0], Name: rest[1]}, nil
}

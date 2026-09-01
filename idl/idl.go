package idl

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

//go:embed *.thrift
var Files embed.FS

type Spec struct {
	Service string
	Methods []Method
	Structs map[string]*Struct
}

type Method struct {
	Name       string
	Req        string
	Resp       string
	HTTPMethod string // from agw.method; empty = RPC only
	URI        string // from agw.uri
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

func ParseEmbedded(name string) (*Spec, error) {
	b, err := Files.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return ParseString(string(b))
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
		if strings.HasPrefix(line, "}") {
			state = stTop
			cur = nil
			continue
		}

		switch state {
		case stTop:
			switch {
			case strings.HasPrefix(line, "namespace ") || strings.HasPrefix(line, "include ") ||
				strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "typedef "):
				continue
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
			m, err := parseMethod(line)
			if err != nil {
				return nil, err
			}
			spec.Methods = append(spec.Methods, m)
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

func parseMethod(line string) (Method, error) {
	line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(line, ";"), ","))
	core, annot := splitAnnot(line)
	name, req, resp, err := parseMethodCore(core)
	if err != nil {
		return Method{}, err
	}
	m := Method{Name: name, Req: req, Resp: resp}
	for k, v := range parseKV(annot) {
		switch k {
		case "agw.method":
			m.HTTPMethod = strings.ToUpper(v)
		case "agw.uri":
			m.URI = v
		}
	}
	return m, nil
}

func parseMethodCore(line string) (name, req, resp string, err error) {
	i := strings.IndexByte(line, '(')
	j := strings.LastIndexByte(line, ')')
	if i < 1 || j < i {
		return "", "", "", fmt.Errorf("idl: bad method %q", line)
	}
	before := strings.Fields(strings.TrimSpace(line[:i]))
	inside := strings.TrimSpace(line[i+1 : j])
	after := strings.TrimSpace(line[j+1:])
	req, err = parseReqType(inside)
	if err != nil {
		return "", "", "", fmt.Errorf("idl: bad method %q", line)
	}
	switch len(before) {
	case 1:
		name, resp = before[0], after
	case 2:
		resp, name = before[0], before[1]
	default:
		return "", "", "", fmt.Errorf("idl: bad method %q", line)
	}
	if name == "" || req == "" || resp == "" {
		return "", "", "", fmt.Errorf("idl: bad method %q", line)
	}
	return name, req, resp, nil
}

func parseReqType(inside string) (string, error) {
	inside = strings.TrimSpace(inside)
	if inside == "" {
		return "", fmt.Errorf("empty req")
	}
	if colon := strings.IndexByte(inside, ':'); colon >= 0 {
		rest := strings.Fields(strings.TrimSpace(inside[colon+1:]))
		if len(rest) < 1 {
			return "", fmt.Errorf("empty req")
		}
		return rest[0], nil
	}
	return inside, nil
}

func parseField(line string) (Field, error) {
	line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(line, ";"), ","))
	core, _ := splitAnnot(line)
	if i := strings.Index(core, "="); i >= 0 {
		core = strings.TrimSpace(core[:i])
	}
	colon := strings.IndexByte(core, ':')
	if colon < 1 {
		return Field{}, fmt.Errorf("idl: bad field %q", line)
	}
	id, err := strconv.Atoi(strings.TrimSpace(core[:colon]))
	if err != nil {
		return Field{}, fmt.Errorf("idl: bad field id in %q", line)
	}
	rest := strings.Fields(strings.TrimSpace(core[colon+1:]))
	if len(rest) == 3 && (rest[0] == "required" || rest[0] == "optional") {
		rest = rest[1:]
	}
	if len(rest) != 2 {
		return Field{}, fmt.Errorf("idl: bad field %q", line)
	}
	if rest[0] != "i64" && rest[0] != "string" && rest[0] != "bool" {
		return Field{}, fmt.Errorf("idl: unsupported type %s", rest[0])
	}
	return Field{ID: id, Type: rest[0], Name: rest[1]}, nil
}

// splitAnnot peels a trailing `(k = v, ...)` annotation block.
func splitAnnot(line string) (core, annot string) {
	line = strings.TrimSpace(line)
	if !strings.HasSuffix(line, ")") {
		return line, ""
	}
	depth := 0
	for i := len(line) - 1; i >= 0; i-- {
		switch line[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth != 0 {
				continue
			}
			inner := line[i+1 : len(line)-1]
			if strings.Contains(inner, "=") || strings.Contains(inner, "agw.") {
				return strings.TrimSpace(line[:i]), inner
			}
			return line, ""
		}
	}
	return line, ""
}

func parseKV(annot string) map[string]string {
	out := map[string]string{}
	if annot == "" {
		return out
	}
	for _, p := range strings.Split(annot, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

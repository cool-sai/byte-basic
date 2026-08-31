package generic

import (
	"encoding/json"
	"fmt"

	"minikitex/idl"
	"minikitex/wire"
)

// Encode turns a map / JSON string / raw bytes into the same struct body that
// generated codec would write.
//
//	map[string]any{"id": 1}     — Kitex MapThriftGeneric
//	`{"id":1}`                  — Kitex JSONThriftGeneric
//	[]byte{...already encoded}  — Kitex BinaryThriftGeneric
func Encode(spec *idl.Spec, structName string, v any) ([]byte, error) {
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	st, err := spec.Struct(structName)
	if err != nil {
		return nil, err
	}
	m, err := asMap(v)
	if err != nil {
		return nil, err
	}
	w := wire.NewWriter()
	for _, f := range st.Fields {
		raw, ok := m[f.Name]
		if !ok || raw == nil {
			continue
		}
		if err := writeField(w, f, raw); err != nil {
			return nil, fmt.Errorf("generic encode %s.%s: %w", structName, f.Name, err)
		}
	}
	w.Stop()
	return w.Bytes(), nil
}

func Decode(spec *idl.Spec, structName string, body []byte) (map[string]any, error) {
	st, err := spec.Struct(structName)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(body)
	out := map[string]any{}
	for {
		typ, id, err := r.NextField()
		if err != nil {
			return nil, err
		}
		if typ == wire.TStop {
			break
		}
		f, ok := st.FieldByID(id)
		if !ok {
			if err := r.Skip(typ); err != nil {
				return nil, err
			}
			continue
		}
		val, err := readValue(r, f.Type)
		if err != nil {
			return nil, fmt.Errorf("generic decode %s.%s: %w", structName, f.Name, err)
		}
		out[f.Name] = val
	}
	return out, nil
}

func asMap(v any) (map[string]any, error) {
	switch x := v.(type) {
	case map[string]any:
		return x, nil
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(x), &m); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, fmt.Errorf("generic: want map[string]any or JSON string, got %T", v)
	}
}

func writeField(w *wire.Writer, f idl.Field, v any) error {
	switch f.Type {
	case "i64":
		n, err := asI64(v)
		if err != nil {
			return err
		}
		w.FieldI64(f.ID, n)
	case "string":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("want string, got %T", v)
		}
		w.FieldString(f.ID, s)
	case "bool":
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("want bool, got %T", v)
		}
		w.FieldBool(f.ID, b)
	default:
		return fmt.Errorf("unsupported type %s", f.Type)
	}
	return nil
}

func readValue(r *wire.Reader, typ string) (any, error) {
	switch typ {
	case "i64":
		return r.ReadI64()
	case "string":
		return r.ReadString()
	case "bool":
		return r.ReadBool()
	default:
		return nil, fmt.Errorf("unsupported type %s", typ)
	}
}

func asI64(v any) (int64, error) {
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case float64: // encoding/json
		return int64(n), nil
	case json.Number:
		return n.Int64()
	default:
		return 0, fmt.Errorf("want i64, got %T", v)
	}
}

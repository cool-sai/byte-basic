package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type memBoard struct {
	mu       sync.Mutex
	kinds    []Kind
	items    []Item
	htmls    []KindHTML
	nextKind int64
	nextItem int64
	nextHTML int64
	updated  string
}

func newMemBoard() *memBoard {
	return &memBoard{nextKind: 1, nextItem: 1, nextHTML: 1, kinds: []Kind{}, items: []Item{}, htmls: []KindHTML{}}
}

func (s *memBoard) CreateKind(name string) (Kind, error) {
	name, err := cleanName(name)
	if err != nil {
		return Kind{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.kinds {
		if k.Name == name {
			return k, nil
		}
	}
	k := Kind{ID: s.nextKind, Name: name}
	s.nextKind++
	s.kinds = append(s.kinds, k)
	return k, nil
}

func (s *memBoard) kindLocked(name string) (Kind, error) {
	name = strings.TrimSpace(name)
	for _, k := range s.kinds {
		if k.Name == name {
			return k, nil
		}
	}
	return Kind{}, fmt.Errorf("没有种类 %s", name)
}

func (s *memBoard) AddItem(kind, content, date, typ string, extra json.RawMessage) (Item, error) {
	content, err := cleanContent(content)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.kindLocked(kind)
	if err != nil {
		return Item{}, err
	}
	it := Item{
		ID: s.nextItem, KindID: k.ID, Kind: k.Name,
		Content: content, Type: typ, Date: day, Extra: extraMap(extra),
	}
	s.nextItem++
	s.items = append(s.items, it)
	s.updated = time.Now().Format(time.RFC3339)
	return it, nil
}

func (s *memBoard) UpdateItem(id int64, kind, content, date, typ *string, extra *json.RawMessage) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := -1
	for n := range s.items {
		if s.items[n].ID == id {
			i = n
			break
		}
	}
	if i < 0 {
		return Item{}, os.ErrNotExist
	}
	it := s.items[i]
	if kind != nil {
		k, err := s.kindLocked(*kind)
		if err != nil {
			return Item{}, err
		}
		it.KindID, it.Kind = k.ID, k.Name
	}
	if content != nil {
		body, err := cleanContent(*content)
		if err != nil {
			return Item{}, err
		}
		it.Content = body
	}
	if date != nil {
		day, err := parseDay(*date)
		if err != nil {
			return Item{}, err
		}
		it.Date = day
	}
	if typ != nil {
		t := strings.TrimSpace(*typ)
		if len([]rune(t)) > 64 {
			return Item{}, fmt.Errorf("type 太长")
		}
		it.Type = t
	}
	if extra != nil {
		it.Extra = extraMap(*extra)
	}
	s.items[i] = it
	s.updated = time.Now().Format(time.RFC3339)
	return it, nil
}

func (s *memBoard) DeleteItem(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.items[:0]
	found := false
	for _, it := range s.items {
		if it.ID == id {
			found = true
			continue
		}
		next = append(next, it)
	}
	if !found {
		return os.ErrNotExist
	}
	s.items = next
	s.updated = time.Now().Format(time.RFC3339)
	return nil
}

func (s *memBoard) ListItems(kind string, page, pageSize int) (ListPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.kindLocked(kind)
	if err != nil {
		return ListPage{}, err
	}
	page, pageSize = pageArgs(page, pageSize)
	var all []Item
	for _, it := range s.items {
		if it.KindID == k.ID {
			all = append(all, it)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Date != all[j].Date {
			return all[i].Date > all[j].Date
		}
		return all[i].ID > all[j].ID
	})
	total := len(all)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := append([]Item{}, all[start:end]...)
	if h, err := s.htmlLocked(k.ID); err == nil {
		applyHTML(out, h.HTML)
	}
	return ListPage{Items: out, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *memBoard) CreateKindHTML(kind, html string) (KindHTML, error) {
	html, err := parseKindHTML(html)
	if err != nil {
		return KindHTML{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.kindLocked(kind)
	if err != nil {
		return KindHTML{}, err
	}
	if _, err := s.htmlLocked(k.ID); err == nil {
		return KindHTML{}, fmt.Errorf("这个种类已经有 html 片段")
	}
	h := KindHTML{ID: s.nextHTML, KindID: k.ID, Kind: k.Name, HTML: html}
	s.nextHTML++
	s.htmls = append(s.htmls, h)
	return h, nil
}

func (s *memBoard) UpdateKindHTML(kind, html string) (KindHTML, error) {
	html, err := parseKindHTML(html)
	if err != nil {
		return KindHTML{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.kindLocked(kind)
	if err != nil {
		return KindHTML{}, err
	}
	for i := range s.htmls {
		if s.htmls[i].KindID == k.ID {
			s.htmls[i].HTML = html
			s.htmls[i].Kind = k.Name
			return s.htmls[i], nil
		}
	}
	return KindHTML{}, fmt.Errorf("这个种类还没有 html 片段")
}

func (s *memBoard) DeleteKindHTML(kind string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.kindLocked(kind)
	if err != nil {
		return err
	}
	next := s.htmls[:0]
	found := false
	for _, h := range s.htmls {
		if h.KindID == k.ID {
			found = true
			continue
		}
		next = append(next, h)
	}
	if !found {
		return fmt.Errorf("这个种类还没有 html 片段")
	}
	s.htmls = next
	return nil
}

func (s *memBoard) GetKindHTML(kind string) (KindHTML, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.kindLocked(kind)
	if err != nil {
		return KindHTML{}, err
	}
	h, err := s.htmlLocked(k.ID)
	if err != nil {
		return KindHTML{}, fmt.Errorf("这个种类还没有 html 片段")
	}
	return h, nil
}

func (s *memBoard) htmlLocked(kindID int64) (KindHTML, error) {
	for _, h := range s.htmls {
		if h.KindID == kindID {
			return h, nil
		}
	}
	return KindHTML{}, os.ErrNotExist
}

func (s *memBoard) Summary() (Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Summary{Kinds: []KindCount{}, UpdatedAt: s.updated}
	counts := map[int64]int{}
	for _, it := range s.items {
		counts[it.KindID]++
		out.Total++
	}
	for _, k := range s.kinds {
		out.Kinds = append(out.Kinds, KindCount{ID: k.ID, Name: k.Name, Count: counts[k.ID]})
	}
	return out, nil
}

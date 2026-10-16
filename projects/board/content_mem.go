package main

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// memContent 是 ContentStore 的内存实现，测试用，语义与 MySQL 版一致。
type memContent struct {
	mu      sync.Mutex
	nextID  int64
	nodes   map[int64]*memNode
	pages   map[string][]memPageVer
	current map[string]int
}

type memNode struct {
	Node
	deleted bool
	seq     int64
}

type memPageVer struct {
	ver   int
	files map[string]string
	at    string
}

func newMemContent() *memContent {
	return &memContent{nodes: map[int64]*memNode{}, pages: map[string][]memPageVer{}, current: map[string]int{}}
}

func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

func (s *memContent) liveLocked(id int64) (*memNode, error) {
	n, ok := s.nodes[id]
	if !ok || n.deleted {
		return nil, errNotFound
	}
	return n, nil
}

func (s *memContent) PutNode(in NodeInput) (Node, error) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	var parent *Node
	if in.ParentID != 0 {
		p, err := s.liveLocked(in.ParentID)
		if err != nil {
			return Node{}, err
		}
		parent = &p.Node
	}
	if err := checkModuleSlug(in.Type, slug); err != nil {
		return Node{}, err
	}
	if err := checkParent(in.Type, parent); err != nil {
		return Node{}, err
	}
	now := nowStr()
	// 按 (parent_id, slug) upsert；软删除过的同名节点复活并覆盖。
	for _, n := range s.nodes {
		if n.ParentID == in.ParentID && n.Slug == slug {
			n.Type, n.Title, n.Data, n.SchemaVersion, n.UpdatedAt = in.Type, in.Title, data, sv, now
			n.deleted = false
			return n.Node, nil
		}
	}
	s.nextID++
	n := &memNode{Node: Node{ID: s.nextID, ParentID: in.ParentID, Type: in.Type, Slug: slug, Title: in.Title,
		Data: data, SchemaVersion: sv, CreatedAt: now, UpdatedAt: now}, seq: s.nextID}
	s.nodes[n.ID] = n
	return n.Node, nil
}

func (s *memContent) UpdateNode(id int64, title *string, data *json.RawMessage, schemaVersion *int) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.liveLocked(id)
	if err != nil {
		return Node{}, err
	}
	if data != nil {
		d, err := cleanNodeData(*data)
		if err != nil {
			return Node{}, err
		}
		n.Data = d
	}
	if schemaVersion != nil {
		sv, err := cleanSchemaVersion(*schemaVersion)
		if err != nil {
			return Node{}, err
		}
		n.SchemaVersion = sv
	}
	if title != nil {
		n.Title = *title
	}
	n.UpdatedAt = nowStr()
	return n.Node, nil
}

func (s *memContent) DeleteNode(id int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.liveLocked(id); err != nil {
		return 0, err
	}
	count := 0
	queue := []int64{id}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if n := s.nodes[cur]; n != nil && !n.deleted {
			n.deleted = true
			count++
		}
		for _, c := range s.nodes {
			if c.ParentID == cur && !c.deleted {
				queue = append(queue, c.ID)
			}
		}
	}
	return count, nil
}

func (s *memContent) GetNode(id int64) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.liveLocked(id)
	if err != nil {
		return Node{}, err
	}
	return n.Node, nil
}

func (s *memContent) FindChild(parentID int64, typ, slug string) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.nodes {
		if !n.deleted && n.ParentID == parentID && n.Type == typ && n.Slug == slug {
			return n.Node, nil
		}
	}
	return Node{}, errNotFound
}

func (s *memContent) ListChildren(parentID int64, typ string, page, size int) (NodePage, error) {
	page, size = nodePageArgs(page, size)
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []*memNode
	for _, n := range s.nodes {
		if !n.deleted && n.ParentID == parentID && n.Type == typ {
			all = append(all, n)
		}
	}
	// 与 MySQL 版一致：新到旧（created_at DESC, id DESC）。
	sort.Slice(all, func(i, j int) bool { return all[i].seq > all[j].seq })
	out := NodePage{Items: []Node{}, Page: page, PageSize: size, Total: len(all)}
	for i := (page - 1) * size; i < len(all) && i < page*size; i++ {
		out.Items = append(out.Items, all[i].Node)
	}
	return out, nil
}

func (s *memContent) PutPage(p string, files map[string]string) (PageVersion, error) {
	p, err := cleanPagePath(p)
	if err != nil {
		return PageVersion{}, err
	}
	files, err = cleanPageFiles(files)
	if err != nil {
		return PageVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := len(s.pages[p]) + 1
	s.pages[p] = append(s.pages[p], memPageVer{ver: v, files: files, at: nowStr()})
	s.current[p] = v
	return PageVersion{Path: p, Version: v, Files: sortedKeys(files), Current: true, CreatedAt: s.pages[p][v-1].at}, nil
}

func (s *memContent) ListPageVersions(p string) ([]PageVersion, error) {
	p, err := cleanPagePath(p)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.pages[p]
	if len(vs) == 0 {
		return nil, errNotFound
	}
	out := make([]PageVersion, 0, len(vs))
	for i := len(vs) - 1; i >= 0; i-- {
		out = append(out, PageVersion{Path: p, Version: vs[i].ver, Files: sortedKeys(vs[i].files), Current: vs[i].ver == s.current[p], CreatedAt: vs[i].at})
	}
	return out, nil
}

func (s *memContent) RollbackPage(p string, version int) (PageVersion, error) {
	p, err := cleanPagePath(p)
	if err != nil {
		return PageVersion{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.pages[p]
	if version < 1 || version > len(vs) {
		return PageVersion{}, errNotFound
	}
	s.current[p] = version
	v := vs[version-1]
	return PageVersion{Path: p, Version: v.ver, Files: sortedKeys(v.files), Current: true, CreatedAt: v.at}, nil
}

func (s *memContent) PageFiles(p string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.current[p]
	if cur == 0 {
		return nil, errNotFound
	}
	return s.pages[p][cur-1].files, nil
}

func (s *memContent) ListPagePaths() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []string{}
	for p, cur := range s.current {
		if cur > 0 {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *memContent) MoveNode(id, parentID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.liveLocked(id)
	if err != nil {
		return err
	}
	if n.ParentID == parentID {
		return nil
	}
	for _, o := range s.nodes { // 与 MySQL uk_parent_slug 一致：软删除的也占名
		if o.ParentID == parentID && o.Slug == n.Slug {
			return errConflict
		}
	}
	n.ParentID = parentID
	return nil
}

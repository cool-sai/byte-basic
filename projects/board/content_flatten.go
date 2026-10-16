package main

import "log"

// legacyList：旧版的 list 节点类型。只在迁移里出现，其他代码已没有 list 概念。
const legacyList = "list"

// flattenLists 把旧数据 module → list → detail 迁成 module → detail，然后软删除空 list（幂等，可重复执行）。
// detail 与 module 下已有节点同名时跳过并记日志，该 list 也保留不删，留给人工处理。
func flattenLists(c ContentStore) (moved, skipped int, err error) {
	all := func(parent int64, typ string) ([]Node, error) {
		var out []Node
		for page := 1; ; page++ {
			p, err := c.ListChildren(parent, typ, page, 100)
			if err != nil {
				return nil, err
			}
			out = append(out, p.Items...)
			if len(p.Items) == 0 || page*p.PageSize >= p.Total {
				return out, nil
			}
		}
	}
	mods, err := all(0, NodeModule)
	if err != nil {
		return 0, 0, err
	}
	for _, m := range mods {
		lists, err := all(m.ID, legacyList)
		if err != nil {
			return moved, skipped, err
		}
		for _, l := range lists {
			ds, err := all(l.ID, NodeDetail)
			if err != nil {
				return moved, skipped, err
			}
			left := 0
			for _, d := range ds {
				switch err := c.MoveNode(d.ID, m.ID); err {
				case nil:
					moved++
				case errConflict:
					skipped++
					left++
					log.Printf("flatten: skip detail %d %q: module %q already has that slug", d.ID, d.Slug, m.Slug)
				default:
					return moved, skipped, err
				}
			}
			if left == 0 {
				if _, err := c.DeleteNode(l.ID); err != nil {
					return moved, skipped, err
				}
			}
		}
	}
	return moved, skipped, nil
}

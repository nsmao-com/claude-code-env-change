package main

// YAML 配置的局部编辑：用 yaml.v3 的节点树读写，只改指定的键，其余内容与注释保留
// （Hermes、MiniMax Code、Mister Morph、omp 等 Agent 的配置是 YAML）。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlDoc struct {
	path string
	root *yaml.Node // 文档里的顶层映射
	doc  *yaml.Node
}

func loadYAMLDoc(path string) (*yamlDoc, error) {
	d := &yamlDoc{path: path}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) > 0 {
		var doc yaml.Node
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s 不是有效的 YAML，未做任何修改: %v", path, err)
		}
		if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
			d.doc, d.root = &doc, doc.Content[0]
			return d, nil
		}
		return nil, fmt.Errorf("%s 的顶层不是映射，未做任何修改", path)
	}
	d.root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	d.doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{d.root}}
	return d, nil
}

func yamlChild(m *yaml.Node, key string) (*yaml.Node, int) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, -1
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], i
		}
	}
	return nil, -1
}

// get 取点号路径上的节点
func (d *yamlDoc) get(path string) *yaml.Node {
	cur := d.root
	for _, k := range strings.Split(path, ".") {
		next, _ := yamlChild(cur, k)
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// value 取路径上的值（解码成普通的 Go 值）
func (d *yamlDoc) value(path string) (any, bool) {
	n := d.get(path)
	if n == nil {
		return nil, false
	}
	var v any
	if n.Decode(&v) != nil {
		return nil, false
	}
	return v, true
}

func (d *yamlDoc) str(path string) string {
	v, _ := d.value(path)
	s, _ := v.(string)
	return s
}

// set 设置路径上的值，缺的中间层自动建成映射
func (d *yamlDoc) set(path string, v any) error {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return err
	}
	keys := strings.Split(path, ".")
	cur := d.root
	for i, k := range keys {
		child, at := yamlChild(cur, k)
		if i == len(keys)-1 {
			if at >= 0 {
				// 换值时保留原来的注释
				old := cur.Content[at+1]
				n.HeadComment, n.LineComment, n.FootComment = old.HeadComment, old.LineComment, old.FootComment
				cur.Content[at+1] = &n
			} else {
				cur.Content = append(cur.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &n)
			}
			return nil
		}
		if child == nil || child.Kind != yaml.MappingNode {
			child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if at >= 0 {
				cur.Content[at+1] = child
			} else {
				cur.Content = append(cur.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, child)
			}
		}
		cur = child
	}
	return nil
}

// del 删除路径上的键；删完变空的上层映射一并去掉
func (d *yamlDoc) del(path string) {
	keys := strings.Split(path, ".")
	chain := []*yaml.Node{d.root} // chain[i] 是 keys[i] 所在的映射
	cur := d.root
	for _, k := range keys[:len(keys)-1] {
		next, _ := yamlChild(cur, k)
		if next == nil || next.Kind != yaml.MappingNode {
			return
		}
		chain = append(chain, next)
		cur = next
	}
	for i := len(keys) - 1; i >= 0; i-- {
		p := chain[i]
		if _, at := yamlChild(p, keys[i]); at >= 0 {
			p.Content = append(p.Content[:at], p.Content[at+2:]...)
		}
		if len(p.Content) > 0 || i == 0 {
			return
		}
		// p 删空了：下一轮在上一层删掉指向它的键
	}
}

// stash 把路径上的原值（没有则为 null）记进 prev
func (d *yamlDoc) stash(prev map[string]json.RawMessage, name, path string) {
	if _, done := prev[name]; done {
		return
	}
	if v, ok := d.value(path); ok {
		b, _ := json.Marshal(v)
		prev[name] = b
		return
	}
	prev[name] = json.RawMessage("null")
}

// restore 还原 stash 记下的值
func (d *yamlDoc) restore(prev map[string]json.RawMessage, name, path string) error {
	raw, ok := prev[name]
	if !ok {
		return nil
	}
	if string(raw) == "null" {
		d.del(path)
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return d.set(path, v)
}

// save 写回文件（先备份）；内容删空时删除文件
func (d *yamlDoc) save() error {
	if len(d.root.Content) == 0 {
		if _, err := backupFile(d.path); err != nil {
			return err
		}
		if err := os.Remove(d.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(d.doc); err != nil {
		return err
	}
	_ = enc.Close()
	return writeAgentFile(d.path, buf.Bytes())
}

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"text/template"
)

const nodeTmpl = `// Auto generated code; Do NOT EDIT!
// Author benzheng2121@126.com

package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

func genXConcSklUniqueNode[K infra.OrderedKey, V any](
	key K,
	val V,
	lvl int32,
) *xConcSklNode[K, V] {
	switch lvl {
{{- range $i, $val := seq .SklMaxLevel }}
	case {{ $val }}: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [{{ $val }}]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
{{- end }}
	default:
	}
	panic("unable to generate ")
}

func genXConcSklLinkedListNode[K infra.OrderedKey, V any](
	key K,
	val V,
	lvl int32,
) *xConcSklNode[K, V] {
	switch lvl {
{{- range $i, $val := seq .SklMaxLevel }}
	case {{ $val }}: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [{{ $val }}]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
{{- end }}
	default:
	}
	panic("unable to generate ")
}

func genXConcSklRbtreeNode[K infra.OrderedKey, V any](
	key K,
	val V,
	vcmp SklValComparator[V],
	lvl int32,
) *xConcSklNode[K, V] {
	switch lvl {
{{- range $i, $val := seq .SklMaxLevel }}
	case {{ $val }}: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [{{ $val }}]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
{{- end }}
	default:
	}
	panic("unable to generate ")
}
`

func seq(n int) []int {
	seq := make([]int, n-1)
	for i := 1; i < n; i++ {
		seq[i-1] = i
	}
	return seq
}

// In vscode env, please run the debug mode to gen code.
// Because the debug mode will create a binary under current
// directory, it is easy to convert relative path to abs path.

func main() {
	var sklMaxLevel int = 32

	tmpl := template.Must(template.New("xConcSkl").
		Funcs(map[string]any{
			"seq": seq,
		}).
		Parse(nodeTmpl),
	)
	path, err := filepath.Abs("../../x_conc_skl_node_gen.go")
	if err != nil {
		panic(err)
	}

	f, err := os.Create(path)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	type tmplData struct {
		SklMaxLevel int
	}
	data := tmplData{SklMaxLevel: sklMaxLevel}
	if err = tmpl.Execute(w, data); err != nil {
		panic(err)
	}
	w.Flush()
}

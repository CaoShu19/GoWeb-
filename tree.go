package csgo

import "strings"

type treeNode struct {
	name       string
	children   []*treeNode
	routerName string
	isEnd      bool
}

//put path:/user/get/:id
func (t *treeNode) Put(path string) {
	root := t
	strs := strings.Split(path, "/")
	//所有
	for index, name := range strs {
		if index == 0 {
			continue
		}
		children := t.children
		isMatch := false
		for _, node := range children {
			//判断所有子树路径是否和传入的路径相匹配，如果有对应匹配
			//就说明树中对应存在，直接跳过循环
			if node.name == name {
				isMatch = true
				t = node
				break
			}
		}
		//如果子节点没有和相应的路径匹配，那么就创建节点，并且放入树中
		if !isMatch {
			isEnd := false
			if index == len(strs)-1 {
				isEnd = true
			}

			node := &treeNode{name: name, children: make([]*treeNode, 0)}
			children = append(children, node)
			t.children = children
			t.isEnd = isEnd
			t = node
		}

	}
	t = root
}

// Get 获得路径树的叶子节点
func (t *treeNode) Get(path string) *treeNode {

	strs := strings.Split(path, "/")
	routerName := ""

	for index, name := range strs {
		if index == 0 {
			continue
		}
		children := t.children
		isMatch := false
		for _, node := range children {
			//判断所有子树路径是否和传入的路径相匹配，如果有对应匹配
			//就说明树中对应存在，直接跳过循环
			if node.name == name ||
				node.name == "*" ||
				strings.Contains(node.name, ":") {

				isMatch = true

				routerName += "/" + node.name
				t = node
				node.routerName = routerName
				if index == len(strs)-1 {

					return node
				}
				break
			}
		}
		if !isMatch {
			for _, node := range children {

				// /user/** 遇到**都能匹配
				if node.name == "**" {
					routerName += "/" + node.name
					node.routerName = routerName

					return node
				}
			}
		}
	}
	return nil
}

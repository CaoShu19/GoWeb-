package csgo

import (
	"fmt"
	"testing"
)

func TestTreeNode(t *testing.T) {

	root := &treeNode{name: "/", children: make([]*treeNode, 0)}
	root.Put("/user/get/:id")
	root.Put("/user/create/user")
	root.Put("/user/create/userT")
	root.Put("/order/get/sss")

	n := root.Get("/user/get/1")
	fmt.Println(n)
	n = root.Get("/user/create/user")
	fmt.Println(n)
	n = root.Get("/user/create/userT")
	fmt.Println(n)
	n = root.Get("/order/get/sss")
	fmt.Println(n)
}

package binding

import (
	"fmt"
	"github.com/go-playground/validator"
	"reflect"
	"strings"
	"sync"
)

// StructValidator 验证器接口
type StructValidator interface {
	// ValidateStruct 结构体验证，如果错误返回对应的错误信息
	ValidateStruct(any) error
	// Engine 返回对应使用的验证器
	Engine() any
}

//具体的验证器结构
type defaultValidator struct {
	//单例表示
	one      sync.Once
	validate *validator.Validate
}

// Validator 采用默认的验证器
var Validator StructValidator = &defaultValidator{}

// ValidateStruct 默认的验证器实现了验证器接口
func (d *defaultValidator) ValidateStruct(obj any) error {
	of := reflect.ValueOf(obj)
	switch of.Kind() {
	case reflect.Pointer:
		return d.validateStruct(of.Elem().Interface())
	case reflect.Struct:
		return d.ValidateStruct(obj)
	case reflect.Slice, reflect.Array:
		count := of.Len()
		sliceValidationError := make(SliceValidationError, 0)
		for i := 0; i < count; i++ {
			if err := d.validateStruct(of.Index(i).Interface()); err != nil {
				sliceValidationError = append(sliceValidationError, err)
			}
		}
		if len(sliceValidationError) == 0 {
			return nil
		}
		return sliceValidationError
	}
	return nil
}

// 将obj转换成struct后才执行插件校验，如果obj是切片，那么循环执行
func validate(obj any) error {
	return Validator.ValidateStruct(obj)
}

// Engine 默认的验证器实现了接口方法
func (d *defaultValidator) Engine() any {
	d.lazyInit()
	return d.validate
}

// 通过同步工具aync实现单例创建验证器
func (d *defaultValidator) lazyInit() {
	d.one.Do(func() {
		d.validate = validator.New()
	})
}

func (d *defaultValidator) validateStruct(obj any) error {
	d.lazyInit()
	return d.validate.Struct(obj)
}

type SliceValidationError []error

func (err SliceValidationError) Error() string {
	n := len(err)
	switch n {
	case 0:
		return ""
	default:
		var b strings.Builder
		if err[0] != nil {
			fmt.Fprintf(&b, "[%d]: %s", 0, err[0].Error())
		}
		if n > 1 {
			for i := 1; i < n; i++ {
				if err[i] != nil {
					b.WriteString("\n")
					fmt.Fprintf(&b, "[%d]: %s", i, err[i].Error())
				}
			}
		}
		return b.String()
	}
}

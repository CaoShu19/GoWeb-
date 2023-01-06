package binding

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

type jsonBinding struct {
	DisallowUnknownFields bool
	IsValidate            bool
}

func (b jsonBinding) Name() string {
	return "json"
}

func (b jsonBinding) Bind(r *http.Request, obj any) error {
	//从请求中获得body
	body := r.Body
	if body == nil {
		return errors.New("body is nil")
	}
	//将body进行解码
	decoder := json.NewDecoder(body)
	//是否开启参数未知字段
	if b.DisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if b.IsValidate {
		//判断json参数是否足够
		err := validateParam(obj, decoder)
		if err != nil {
			return err
		}
	} else {
		//将解码后的body转化为obj类型
		err := decoder.Decode(obj)
		//使用第三方组件做校验
		if err != nil {
			return err
		}
	}
	return validate(obj)
}

func validateParam(obj any, decoder *json.Decoder) error {
	//先将json参数解析map 根据map的key进行对比
	//通过反射获得obj的类型及其成员,判断是否为指针类型
	valueOf := reflect.ValueOf(obj)
	//只有指针类型才能进行引用
	if valueOf.Kind() != reflect.Pointer {
		return errors.New("no ptr kind")
	}
	//获得类型的成员
	elem := valueOf.Elem().Interface()
	//获得结构体的成员
	of := reflect.ValueOf(elem)

	switch of.Kind() {
	case reflect.Struct:
		return checkParam(of, obj, decoder)
	case reflect.Slice, reflect.Array:
		elem := of.Type().Elem()
		if elem.Kind() == reflect.Struct {
			return checkParamSlice(elem, obj, decoder)
		}
	default:
		_ = decoder.Decode(obj)
	}
	return nil
}

func checkParamSlice(of reflect.Type, obj any, decoder *json.Decoder) error {
	mapValue := make([]map[string]interface{}, 0)
	//将请求中参数解码到map
	_ = decoder.Decode(&mapValue)
	for i := 0; i < of.NumField(); i++ {
		//获得参数的字段
		field := of.Field(i)
		//获得字段的名字
		name := field.Name
		//获得属性上的json标签 比如如下name和 password 会被获取到
		//type User struct {
		//	Name     string `xml:"name" json:"name"`
		//	Password string `xml:"pwd" json:"password" csgo:"required"`
		//}
		jsonName := field.Tag.Get("json")
		if jsonName != "" {
			name = jsonName
		}
		required := field.Tag.Get("csgo")
		for _, v := range mapValue {
			value := v[name]
			//如果传入的参数字段和定义结构体的属性不匹配，那么value就为空
			//如果定义的结构体的属性后面加了tag为required，那么必须要有此required才进行报错
			//简单来说就是 对于加了required标签的字段是必须传入参数才能不报错
			if value == nil && required == "required" {
				return errors.New(fmt.Sprintf("field [%s] is not exist", name))
			}
		}
	}
	b, _ := json.Marshal(mapValue)
	json.Unmarshal(b, obj)
	return nil
}

func checkParam(of reflect.Value, obj any, decoder *json.Decoder) error {
	mapValue := make(map[string]interface{})
	//将请求中参数解码到map
	_ = decoder.Decode(&mapValue)
	for i := 0; i < of.NumField(); i++ {
		//获得参数的字段
		field := of.Type().Field(i)
		//获得字段的名字
		name := field.Name
		//获得属性上的json标签 比如如下name和 password 会被获取到
		//type User struct {
		//	Name     string `xml:"name" json:"name"`
		//	Password string `xml:"pwd" json:"password" csgo:"required"`
		//}
		jsonName := field.Tag.Get("json")
		fmt.Println("jsonName:", jsonName)
		if jsonName != "" {
			name = jsonName
		}
		required := field.Tag.Get("csgo")
		value := mapValue[name]
		//如果传入的参数字段和定义结构体的属性不匹配，那么value就为空
		//如果定义的结构体的属性后面加了tag为required，那么必须要有此required才进行报错
		//简单来说就是 对于加了required标签的字段是必须传入参数才能不报错
		if value == nil && required == "required" {
			return errors.New(fmt.Sprintf("field [%s] is not exist", jsonName))
		}
		fmt.Println("fieldName:", name)
	}
	b, _ := json.Marshal(mapValue)
	json.Unmarshal(b, obj)
	return nil
}

package csgo

import (
	"errors"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"web/csgo/binding"
	csLog "web/csgo/log"
	"web/csgo/render"
)

const defaultMaxMemory = 32 << 20 //32M
type Context struct {
	W http.ResponseWriter
	R *http.Request
	//加载资源
	engine *Engine
	//存放get请求参数
	queryCache url.Values
	//存放post表单参数
	formCache url.Values
	//是否校验json参数中是否有未知字段
	DisallowUnknownFields bool
	//是否开启校验json参数是否不够（没有满足对应的结构体）
	IsValidate bool
	//status
	StatusCode int

	//日志打印
	Logger *csLog.Logger
	//加密
	Keys map[string]any
	//读写锁
	mu sync.RWMutex
	//
	sameSite http.SameSite
}

func (c *Context) SetSameSite(s http.SameSite) {
	c.sameSite = s
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}
	c.Keys[key] = value
	c.mu.Unlock()
}
func (c *Context) Get(key string) (value any, ok bool) {
	c.mu.RLock()
	value, ok = c.Keys[key]
	c.mu.RUnlock()
	return
}
func (c *Context) SetBasicAuth(username, password string) {
	c.R.Header.Set("Authorization", "Basic "+BasicAuth(username, password))
}

// GetDefaultQuery 如果没传入参数，那么我们返回服务器定义的参数
func (c *Context) GetDefaultQuery(key, defaultValue string) string {
	values, b := c.GetQueryArray(key)
	if !b {
		return defaultValue
	}
	return values[0]
}

func (c *Context) GetQuery(key string) any {
	c.initQueryCache()
	return c.queryCache.Get(key)
}
func (c *Context) GetQueryArray(key string) ([]string, bool) {
	c.initQueryCache()
	vales, ok := c.queryCache[key]
	return vales, ok
}
func (c *Context) QueryArray(key string) []string {
	c.initQueryCache()
	vales, _ := c.queryCache[key]
	return vales
}

func (c *Context) initQueryCache() {
	if c.R != nil {
		//从请求中获得参数，存入到queryCache
		c.queryCache = c.R.URL.Query()
	} else {
		c.queryCache = url.Values{}
	}
}
func (c *Context) QueryMap(key string) (dicts map[string]string) {
	dicts, _ = c.GetQueryMap(key)
	return
}

func (c *Context) GetQueryMap(key string) (map[string]string, bool) {
	c.initQueryCache()
	return c.get(c.queryCache, key)
}
func (c *Context) get(cache map[string][]string, key string) (map[string]string, bool) {
	//user[id]=1&user[name]=张三
	//map[string][]string --> user[id] 为key ; 1 为value

	dicts := make(map[string]string)
	exist := false
	for k, value := range cache {
		//获取k中'['的位置索引
		//对key 和 请求中的参数进行对比
		if i := strings.IndexByte(k, '['); i >= 1 && k[0:i] == key {
			if j := strings.IndexByte(k[i+1:], ']'); j >= 1 {
				exist = true
				dicts[k[i+1:][:j]] = value[0]
			}
		}
	}
	return dicts, exist
}
func (c *Context) initPostFormCache() {
	if c.R != nil {
		//对表单文件进行解析
		if err := c.R.ParseMultipartForm(defaultMaxMemory); err != nil {
			//如果表单不是文件，那么就会报错，但有时候是正常异常
			if errors.Is(err, http.ErrNotMultipart) {
				//如果异常不是解析异常 打印异常
				log.Println(err)
			}
		}
		//从post请求中获得参数，存入到formCache
		c.formCache = c.R.PostForm
	} else {
		c.formCache = url.Values{}
	}
}

func (c *Context) GetPostForm(key string) (string, bool) {
	if values, ok := c.GetPostFormArray(key); ok {
		return values[0], ok
	}
	return "", false
}

func (c *Context) PostFormArray(key string) (values []string) {
	values, _ = c.GetPostFormArray(key)
	return
}

func (c *Context) GetPostFormArray(key string) (values []string, ok bool) {
	c.initPostFormCache()
	values, ok = c.formCache[key]
	return
}

func (c *Context) GetPostFormMap(key string) (map[string]string, bool) {
	c.initPostFormCache()
	return c.get(c.formCache, key)
}

func (c *Context) PostFormMap(key string) (dicts map[string]string) {
	dicts, _ = c.GetPostFormMap(key)
	return
}

func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	file, header, err := c.R.FormFile(name)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	defer file.Close()
	return header, nil
}

func (c *Context) FormFiles(name string) ([]*multipart.FileHeader, error) {

	multipartForm, err := c.MultipartForm()
	if err != nil {
		return make([]*multipart.FileHeader, 0), nil
	}
	return multipartForm.File[name], nil
}
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// MultipartForm 获得form中所有解析
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.R.ParseMultipartForm(defaultMaxMemory)
	return c.R.MultipartForm, err
}

func (c *Context) HTML(status int, html string) {
	c.Render(status, &render.HTML{IsTemplate: false, Data: html})
}

func (c *Context) HTMLTemplate(name string, data any, filenames ...string) error {

	c.W.Header().Set("Content-Type", "text/html;charset=utf-8")

	t := template.New(name)

	t, err := t.ParseFiles(filenames...)
	if err != nil {
		return err
	}
	t.Execute(c.W, data)

	return err
}

func (c *Context) HTMLTemplateGlob(name string, data any, pattern string) error {

	c.W.Header().Set("Content-Type", "text/html;charset=utf-8")

	t := template.New(name)
	//解析模板
	t, err := t.ParseGlob(pattern)
	if err != nil {
		return err
	}
	t.Execute(c.W, data)

	return err
}

// Template 使用全局加载模板
func (c *Context) Template(name string, data any) error {

	return c.Render(http.StatusOK, &render.HTML{
		Data:       data,
		IsTemplate: true,
		Template:   c.engine.HTMLRender.Template,
		Name:       name,
	})

}

func (c *Context) JSON(status int, data any) error {
	err := c.Render(status, &render.JSON{Data: data})
	return err
}

func (c *Context) XML(status int, data any) error {
	//调用通用接口进行渲染
	err := c.Render(status, &render.XML{Data: data})
	return err
}

func (c *Context) File(fileName string) {
	http.ServeFile(c.W, c.R, fileName)
}

func (c *Context) FileAttachment(filepath, fileName string) {
	if isASCII(fileName) {
		c.W.Header().Set("Content-Type", `attachment; filename="`+fileName+`"`)
	} else {
		c.W.Header().Set("Content-Type", `attachment; filename="`+url.QueryEscape(fileName))
	}
	http.ServeFile(c.W, c.R, filepath)
}
func (c *Context) FileFromFS(filepath string, fs http.FileSystem) {
	defer func(old string) {
		c.R.URL.Path = old
	}(c.R.URL.Path)

	c.R.URL.Path = filepath

	http.FileServer(fs).ServeHTTP(c.W, c.R)

}
func (c *Context) Redirect(status int, url string) error {
	return c.Render(status, &render.Redirect{
		Code:     status,
		Request:  c.R,
		Location: url,
	})
}

func (c *Context) String(status int, format string, values ...any) error {
	//调用通用接口进行渲染
	err := c.Render(status, &render.String{Format: format, Data: values})
	return err
}

func (c *Context) Render(status int, r render.Render) error {

	err := r.Render(c.W, status)
	c.StatusCode = status
	return err
}
func (c *Context) BindXml(obj any) error {
	xml := binding.XML
	//用绑定器来处理obj  绑定器现在是json参数验证的作用
	return c.MustBindWith(obj, xml)
}

func (c *Context) BindJson(obj any) error {
	json := binding.JSON
	//用绑定器来处理obj  绑定器现在是json参数验证的作用
	return c.MustBindWith(obj, json)
}

func (c *Context) MustBindWith(obj any, bind binding.Binding) error {
	if err := c.ShouldBind(obj, bind); err != nil {
		// return 400 to behalf index is not match
		c.W.WriteHeader(http.StatusBadRequest)
		return err
	}
	return nil
}
func (c *Context) ShouldBind(obj any, bind binding.Binding) error {
	return bind.Bind(c.R, obj)
}

func (c *Context) Fail(code int, msg string) {
	//简单拼接错误信息 并返回
	c.String(code, msg)
}

func (c *Context) HandleWithError(statusCode int, obj any, err error) {
	if err != nil {
		code, data := c.engine.errorHandler(err)
		c.JSON(code, data)
		return
	}
	c.JSON(statusCode, obj)
}

func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.W, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		SameSite: c.sameSite,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

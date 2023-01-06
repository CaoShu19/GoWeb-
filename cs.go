package csgo

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"web/csgo/config"
	csLog "web/csgo/log"
	"web/csgo/render"
)

const ANY = "ANY"

// HandleFunc 请求和回复 处理方法
type HandleFunc func(ctx *Context)

// MiddlewareFunc 中间件，在处理方法前后进行调用
type MiddlewareFunc func(handleFunc HandleFunc) HandleFunc

type ErrorHandler func(err error) (int, any)

type Engine struct {
	router
	funcMap template.FuncMap
	//引入某个包中的
	HTMLRender render.HTMLRender
	//变量池:
	pool sync.Pool
	//日志
	Logger *csLog.Logger
	//中间件
	middle       []MiddlewareFunc
	errorHandler ErrorHandler
}

//用组来维护uri映射和方法
type routerGroup struct {
	name string
	//一个路径对应一个请求回复处理方法
	handleFuncMap     map[string]map[string]HandleFunc
	middlewareFuncMap map[string]map[string][]MiddlewareFunc
	handlerMethodMap  map[string][]string
	treeNode          *treeNode
	//中间件
	middlewares []MiddlewareFunc
}

// 路由器-->组--> get(key) --> handler
type router struct {
	//一个路由维护路由组
	routerGroups []*routerGroup
	engine       *Engine
}

// Group 创建一个组，并将其放到路由中
func (r *router) Group(name string) *routerGroup {
	routerGroup := &routerGroup{
		name:              name,
		handleFuncMap:     make(map[string]map[string]HandleFunc),
		middlewareFuncMap: make(map[string]map[string][]MiddlewareFunc),
		handlerMethodMap:  make(map[string][]string),
		treeNode:          &treeNode{name: "/", children: make([]*treeNode, 0)},
	}
	//添加中间件
	routerGroup.Use(r.engine.middle...)
	//将路由组放到路由中
	r.routerGroups = append(r.routerGroups, routerGroup)
	return routerGroup
}

func (r *routerGroup) Use(middlewareFunc ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, middlewareFunc...)
}

func (r *routerGroup) MethodHandle(name string, method string, h HandleFunc, ctx *Context) {

	//组全部处理中间件
	if r.middlewares != nil {
		for _, middleware := range r.middlewares {
			h = middleware(h)
		}
	}
	//组路由级别中间件
	middlewareFunc := r.middlewareFuncMap[name][method]
	if middlewareFunc != nil {
		for _, middlewareFunc1 := range middlewareFunc {
			h = middlewareFunc1(h)
		}
	}
	h(ctx)
}

// 将路径和处理方法映射起来，同时把中间件和处理方法也映射上
func (r *routerGroup) handle(name string, handleFunc HandleFunc, method string, middlewareFunc ...MiddlewareFunc) {
	_, ok := r.handleFuncMap[name]
	if !ok {
		r.handleFuncMap[name] = make(map[string]HandleFunc)
		r.middlewareFuncMap[name] = make(map[string][]MiddlewareFunc)
	}
	_, ok = r.handleFuncMap[name][method]
	if ok {
		panic("有重复的路由，请勿两次添加")
	}
	r.handleFuncMap[name][method] = handleFunc
	r.handlerMethodMap[method] = append(r.handlerMethodMap[method], name)

	r.middlewareFuncMap[name][method] = append(r.middlewareFuncMap[name][method], middlewareFunc...)

	r.treeNode.Put(name)
}

func (r *routerGroup) Any(name string, handleFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handleFunc, ANY, middlewareFunc...)
}

func (r *routerGroup) Get(name string, handleFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handleFunc, http.MethodGet, middlewareFunc...)
}

func (r *routerGroup) Post(name string, handleFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handleFunc, http.MethodPost, middlewareFunc...)
}
func (r *routerGroup) Delete(name string, handlerFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handlerFunc, http.MethodDelete, middlewareFunc...)
}
func (r *routerGroup) Put(name string, handlerFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handlerFunc, http.MethodPut, middlewareFunc...)
}
func (r *routerGroup) Patch(name string, handlerFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handlerFunc, http.MethodPatch, middlewareFunc...)
}
func (r *routerGroup) Options(name string, handlerFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handlerFunc, http.MethodOptions, middlewareFunc...)
}
func (r *routerGroup) Head(name string, handlerFunc HandleFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, handlerFunc, http.MethodHead, middlewareFunc...)
}

// New 初始化启动引擎/**
func New() *Engine {
	engine := &Engine{
		router: router{},
	}
	engine.pool.New = func() any {
		return engine.allocateContext()
	}
	return engine
}
func Default() *Engine {
	engine := New()
	//给engine添加一个默认初始化的日志
	engine.Logger = csLog.Default()
	logPath, ok := config.Conf.Log["path"]
	if ok {
		engine.Logger.SetLogPath(logPath.(string))
	}
	//默认对每个组都进行添加日志中间件和错误处理中间件
	engine.Use(Logging, Recovery)
	engine.router.engine = engine
	return engine
}

func (e *Engine) allocateContext() any {
	return &Context{engine: e}
}
func (e *Engine) SetFuncMap(funcMap template.FuncMap) {
	e.funcMap = funcMap
}
func (e *Engine) LoadFuncMap(pattern string) {
	//将html模板加载出来
	t := template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern))
	//将模板放入启动引擎中
	e.SetHtmlTemplate(t)
}
func (e *Engine) LoadFuncMapConf() {
	pattern, ok := config.Conf.Template["pattern"]
	if !ok {
		return
	}
	//将html模板加载出来
	t := template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern.(string)))
	//将模板放入启动引擎中
	e.SetHtmlTemplate(t)
}
func (e *Engine) SetHtmlTemplate(t *template.Template) {
	e.HTMLRender = render.HTMLRender{Template: t}
}

//http通道的修饰，包装成ctx，并且添加了日志处理，和请求处理
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := e.pool.Get().(*Context)
	ctx.W = w
	ctx.R = r
	ctx.Logger = e.Logger
	e.httpRequestHandle(ctx, w, r)
	e.pool.Put(ctx)
}

func (e *Engine) Run() {
	//将路由组中信息和方法处理映射起来
	//for _, group := range e.routerGroups {
	//	for key, value := range group.handleFuncMap {
	//		http.HandleFunc("/"+group.name+key, value)
	//	}
	//}
	http.Handle("/", e)
	//http服务器监听http请求在8111端口上
	err := http.ListenAndServe(":8111", nil)
	if err != nil {
		log.Fatal(err)
	}
}
func (e *Engine) RunTLS(addr, certFile, keyFile string) {
	err := http.ListenAndServeTLS(addr, certFile, keyFile, e.Handler())
	if err != nil {
		log.Fatal(err)
	}
}

//处理请求
func (e *Engine) httpRequestHandle(ctx *Context, w http.ResponseWriter, r *http.Request) {

	method := r.Method

	//处理 路径映射和方法请求方式
	for _, group := range e.routerGroups {
		//拿到请求的路径（不含参数）
		routerName := SubStringLast(r.URL.Path, "/"+group.name)

		node := group.treeNode.Get(routerName)
		if node != nil && !node.isEnd {
			//从group中拿到请求方法

			handle, ok := group.handleFuncMap[node.routerName][ANY]
			if ok {
				//处理通道
				group.MethodHandle(node.routerName, ANY, handle, ctx)
				return
			}
			handle, ok = group.handleFuncMap[node.routerName][method]
			if ok {
				group.MethodHandle(node.routerName, method, handle, ctx)
				return
			}

			//对这种url请求的处理方式没有，返回状态码
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "%s %s not allowed\n", r.RequestURI, method)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "%s not found\n", r.RequestURI)
}

func (e *Engine) Use(middles ...MiddlewareFunc) {
	e.middle = append(e.middle, middles...)
}

func (e *Engine) RegisterErrorHandler(handler ErrorHandler) {
	e.errorHandler = handler
}

func (e *Engine) Handler() http.Handler {
	return e
}

package web

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

var _app *Application
var _once sync.Once

// Callback function
type Callback func(ctx *Context)

// Param struct
type Param struct {
	Key   string
	Value string
}

// Params list
type Params []Param

// Application is type of a web.Application
type Application struct {
	trees       map[string]*node
	middlewares []Callback
	logger      *log.Logger
	paramsPool  sync.Pool
	maxParams   uint16
}

// Create return a singleton web.Application
func Create() *Application {
	_once.Do(func() {
		_app = newApplication()
	})
	return _app
}

// newApplication return a web.Application
func newApplication() *Application {
	app := &Application{
		middlewares: []Callback{},
		logger:      log.New(os.Stdout, "", log.Ldate|log.Ltime),
	}

	return app
}

// Use Add the given callback function to this application.middlewares.
func (app *Application) Use(callback Callback) {
	app.middlewares = append(app.middlewares, callback)
}

// Resource Add the given callback function to this application.middlewares.
func (app *Application) Resource(path string, callback Callback) {
	app.middlewares = append(app.middlewares, callback)
}

// On add event
func (app *Application) On(name string, callback Callback) {

}

// Get method
func (app *Application) Get(path string, callback Callback) {
	app.addRoute(http.MethodGet, path, callback)
}

func (app *Application) addRoute(method, path string, callback Callback) {

	if method == "" {
		panic("method must not be empty")
	}

	if len(path) < 1 || path[0] != '/' {
		panic("path must begin with '/' in path '" + path + "'")
	}

	if callback == nil {
		panic("callback must not be nil")
	}

	if app.trees == nil {
		app.trees = make(map[string]*node)
	}

	root := app.trees[method]

	if root == nil {
		root = new(node)
		app.trees[method] = root
	}

	root.addRoute(path, callback)

	if pc := countParams(path); pc > app.maxParams {
		app.maxParams = pc
	}

	if app.paramsPool.New == nil && app.maxParams > 0 {
		app.paramsPool.New = func() interface{} {
			ps := make(Params, 0, app.maxParams)
			return &ps
		}
	}
}

// ServeHTTP
func (app *Application) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	startTime := time.Now()

	path := r.URL.Path

	if root := app.trees[r.Method]; root != nil {

		if callback, params, _ := root.getValue(path, app.getParams); callback != nil {

			runTime := time.Now()

			ctx := newContext(w, r, params)

			for i := range app.middlewares {
				callback := app.middlewares[i]
				callback(ctx)
			}

			callback(ctx)

			app.logger.Printf("%s %v %s", path, params, runTime.Sub(startTime))

			return
		}
	}

	http.NotFound(w, r)

	// ctx := newContext(w, r)

	// for i := range app.middlewares {
	// 	callback := app.middlewares[i]
	// 	callback(ctx)
	// }

	endTime := time.Now()

	app.logger.Printf("%s %s", path, endTime.Sub(startTime))
}

// ListenAndServe on addr
func (app *Application) ListenAndServe(addr string) error {

	mux := http.NewServeMux()

	mux.Handle("/", app)

	l, err := net.Listen("tcp", addr)

	if err != nil {
		log.Fatal("Listen:", err)
	}

	defer l.Close()

	app.logger.Printf("web.go serving %s\n", l.Addr())

	return http.Serve(l, mux)
}

// ListenAndServeTLS on addr
func (app *Application) ListenAndServeTLS(addr string, tlsConfig *tls.Config) error {

	mux := http.NewServeMux()

	mux.Handle("/", app)

	l, err := tls.Listen("tcp", addr, tlsConfig)

	if err != nil {
		log.Fatal("Listen:", err)
	}

	defer l.Close()

	app.logger.Printf("web.go serving %s\n", l.Addr())

	return http.Serve(l, mux)
}

// Inspect method
func (app *Application) Inspect() string {
	return ""
}

// Log method
func (app *Application) Log(line string) {

}

func (app *Application) getParams() *Params {
	ps := app.paramsPool.Get().(*Params)
	*ps = (*ps)[0:0] // reset slice
	return ps
}

func (app *Application) putParams(ps *Params) {
	if ps != nil {
		app.paramsPool.Put(ps)
	}
}

// newContext return a web.Context
func newContext(w http.ResponseWriter, r *http.Request, params *Params) *Context {

	ctx := &Context{
		Response: &Response{
			w: w,
		},
		Request: &Request{
			r: r,
		},
		Params: params,
	}

	return ctx
}

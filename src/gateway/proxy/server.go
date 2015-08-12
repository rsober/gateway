package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gateway/admin"
	"gateway/config"
	"gateway/db/pools"
	aphttp "gateway/http"
	"gateway/model"
	apvm "gateway/proxy/vm"
	sql "gateway/sql"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx/types"
	"github.com/robertkrimen/otto"
)

// Server encapsulates the proxy server.
type Server struct {
	devMode     bool
	proxyConf   config.ProxyServer
	adminConf   config.ProxyAdmin
	router      *mux.Router
	proxyRouter *proxyRouter
	proxyData   proxyDataSource
	ownDb       *sql.DB // in-application datastore
	dbPools     *pools.Pools
	httpClient  *http.Client
}

// NewServer builds a new proxy server.
func NewServer(conf config.Configuration, ownDb *sql.DB) *Server {
	httpTimeout := time.Duration(conf.Proxy.HTTPTimeout) * time.Second

	var source proxyDataSource
	if conf.Proxy.CacheAPIs {
		source = newCachingProxyDataSource(ownDb)
	} else {
		source = newPassthroughProxyDataSource(ownDb)
	}

	pools := pools.MakePools()
	ownDb.RegisterListener(pools)

	return &Server{
		devMode:    conf.DevMode(),
		proxyConf:  conf.Proxy,
		adminConf:  conf.Admin,
		router:     mux.NewRouter(),
		proxyData:  source,
		ownDb:      ownDb,
		dbPools:    pools,
		httpClient: &http.Client{Timeout: httpTimeout},
	}
}

// Run runs the server.
func (s *Server) Run() {

	// Set up admin
	admin.Setup(s.router, s.ownDb, s.adminConf, s.proxyConf)

	// Set up proxy
	s.proxyRouter = newProxyRouter(s.ownDb)

	s.router.Handle("/{path:.*}",
		aphttp.AccessLoggingHandler(config.Proxy, s.proxyConf.RequestIDHeader,
			aphttp.ErrorCatchingHandler(s.proxyHandlerFunc))).
		MatcherFunc(s.isRoutedToEndpoint)

	s.router.NotFoundHandler = s.accessLoggingNotFoundHandler()

	// Run server
	listen := fmt.Sprintf("%s:%d", s.proxyConf.Host, s.proxyConf.Port)
	log.Printf("%s Server listening at %s", config.Proxy, listen)
	log.Fatalf("%s %v", config.System, http.ListenAndServe(listen, s.router))
}

func (s *Server) isRoutedToEndpoint(r *http.Request, rm *mux.RouteMatch) bool {
	var match mux.RouteMatch
	ok := s.proxyRouter.Match(r, &match)
	if ok {
		context.Set(r, aphttp.ContextMatchKey, &match)
	}
	return ok
}

func (s *Server) proxyHandlerFunc(w http.ResponseWriter, r *http.Request) (httpErr aphttp.Error) {
	start := time.Now()

	match := context.Get(r, aphttp.ContextMatchKey).(*mux.RouteMatch)
	requestID := context.Get(r, aphttp.ContextRequestIDKey).(string)
	logPrefix := context.Get(r, aphttp.ContextLogPrefixKey).(string)
	test, _ := context.Get(r, aphttp.ContextTest).(bool)

	var vm *apvm.ProxyVM

	defer func() {
		if httpErr != nil {
			s.logError(logPrefix, httpErr)
		}
		s.logDuration(vm, logPrefix, start)
	}()

	proxyEndpointID, err := strconv.ParseInt(match.Route.GetName(), 10, 64)
	if err != nil {
		return s.httpError(err)
	}

	proxyEndpoint, err := s.proxyData.Endpoint(proxyEndpointID)
	if err != nil {
		return s.httpError(err)
	}

	libraries, err := s.proxyData.Libraries(proxyEndpoint.APIID)
	if err != nil {
		return s.httpError(err)
	}

	log.Printf("%s [route] %s", logPrefix, proxyEndpoint.Name)

	if r.Method == "OPTIONS" {
		route, err := s.matchingRouteForOptions(proxyEndpoint, r)
		if err != nil {
			return s.httpError(err)
		}
		if !route.HandlesOptions() {
			return s.corsOptionsHandlerFunc(w, r, proxyEndpoint, route, requestID)
		}
	}

	vm, err = apvm.NewVM(logPrefix, w, r, s.proxyConf, s.ownDb, proxyEndpoint, libraries)
	if err != nil {
		return s.httpError(err)
	}

	incomingJSON, err := proxyRequestJSON(r, requestID, match.Vars)
	if err != nil {
		return s.httpError(err)
	}
	vm.Set("__ap_proxyRequestJSON", incomingJSON)
	scripts := []interface{}{
		"var request = JSON.parse(__ap_proxyRequestJSON);",
		"var response = new AP.HTTP.Response();",
	}
	scripts = append(scripts,
		fmt.Sprintf("var session = new AP.Session(%s);",
			strconv.Quote(proxyEndpoint.Environment.SessionName)))

	if _, err := vm.RunAll(scripts); err != nil {
		return s.httpError(err)
	}

	if err = s.runComponents(vm, proxyEndpoint.Components); err != nil {
		return s.httpError(err)
	}

	responseObject, err := vm.Run("response;")
	if err != nil {
		return s.httpError(err)
	}
	responseJSON, err := s.objectJSON(vm, responseObject)
	if err != nil {
		return s.httpError(err)
	}
	response, err := proxyResponseFromJSON(responseJSON)
	if err != nil {
		return s.httpError(err)
	}

	if proxyEndpoint.CORSEnabled {
		s.addCORSCommonHeaders(w, proxyEndpoint)
	}
	response.Headers["Content-Length"] = len(response.Body)
	aphttp.AddHeaders(w.Header(), response.Headers)

	w.WriteHeader(response.StatusCode)
	if test {
		response := aphttp.TestResponse{
			Body: response.Body,
			Log:  vm.Log.String(),
		}

		body, err := json.Marshal(&response)
		if err != nil {
			return s.httpError(err)
		}
		w.Write(body)
	} else {
		w.Write([]byte(response.Body))
	}
	return nil
}

func (s *Server) httpError(err error) aphttp.Error {
	if !s.devMode {
		return aphttp.DefaultServerError()
	}

	return aphttp.NewServerError(err)
}

func (s *Server) objectJSON(vm *apvm.ProxyVM, object otto.Value) (string, error) {
	jsJSON, err := vm.Object("JSON")
	if err != nil {
		return "", err
	}
	result, err := jsJSON.Call("stringify", object)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func (s *Server) accessLoggingNotFoundHandler() http.Handler {
	return aphttp.AccessLoggingHandler(config.Proxy, s.proxyConf.RequestIDHeader,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
}

func (s *Server) runStoredJSONScript(vm *apvm.ProxyVM, jsonScript types.JsonText) error {
	script, err := strconv.Unquote(string(jsonScript))
	if err != nil || script == "" {
		return err
	}
	_, err = vm.Run(script)
	return err
}

func (s *Server) matchingRouteForOptions(endpoint *model.ProxyEndpoint,
	r *http.Request) (*model.ProxyEndpointRoute, error) {
	routes, err := endpoint.GetRoutes()
	if err != nil {
		return nil, err
	}
	for _, proxyRoute := range routes {
		route := &mux.Route{}
		route.Path(proxyRoute.Path)
		methods := proxyRoute.Methods
		if !proxyRoute.HandlesOptions() {
			methods = append(methods, "OPTIONS")
		}
		route.Methods(methods...)
		var match mux.RouteMatch
		if route.Match(r, &match) {
			return proxyRoute, nil
		}
	}
	return nil, errors.New("No route matched")
}

func (s *Server) corsOptionsHandlerFunc(w http.ResponseWriter, r *http.Request,
	endpoint *model.ProxyEndpoint, route *model.ProxyEndpointRoute,
	requestID string) aphttp.Error {

	s.addCORSCommonHeaders(w, endpoint)
	methods := route.Methods
	methods = append(methods, "OPTIONS")
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
	return nil
}

func (s *Server) addCORSCommonHeaders(w http.ResponseWriter,
	endpoint *model.ProxyEndpoint) {

	api := endpoint.API

	w.Header().Set("Access-Control-Allow-Origin", api.CORSAllowOrigin)
	w.Header().Set("Access-Control-Request-Headers", api.CORSRequestHeaders)
	w.Header().Set("Access-Control-Allow-Headers", api.CORSAllowHeaders)
	w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", api.CORSMaxAge))

	if api.CORSAllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

func (s *Server) logError(logPrefix string, err aphttp.Error) {
	errString := "Unknown Error"
	lines := strings.Split(err.String(), "\n")
	if len(lines) > 0 {
		errString = lines[0]
	}
	log.Printf("%s [error] %s", logPrefix, errString)
}

func (s *Server) logDuration(vm *apvm.ProxyVM, logPrefix string, start time.Time) {
	var proxiedRequestsDuration time.Duration
	if vm != nil {
		proxiedRequestsDuration = vm.ProxiedRequestsDuration
	}

	total := time.Since(start)
	processing := total - proxiedRequestsDuration
	log.Printf("%s [time] %v (processing %v, requests %v)",
		logPrefix, total, processing, proxiedRequestsDuration)
}
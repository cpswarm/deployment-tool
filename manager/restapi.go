package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/storage"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/justinas/alice"
	"github.com/rs/cors"
	yaml "gopkg.in/yaml.v2"
)

const (
	// query parameter key/values
	_page          = "page"
	_perPage       = "perPage"
	_sortBy        = "sortBy"
	_sortOrder     = "sortOrder"
	_asc           = "asc"
	_desc          = "desc"
	_time          = "time"
	_target        = "target"
	_task          = "task"
	_stage         = "stage"
	_command       = "command"
	_tags          = "tags"
	_topics        = "topics"
	defaultPage    = 1
	defaultPerPage = 100
)

type restAPI struct {
	manager *manager
	router  *mux.Router
}

type list struct {
	Total   int64       `json:"total"`
	Items   interface{} `json:"items"` // array of anything
	Page    int         `json:"page,omitempty"`
	PerPage int         `json:"perPage,omitempty"`
}

func startRESTAPI(bindAddr string, manager *manager) {

	a := restAPI{
		manager: manager,
	}

	a.setupRouter()

	chain := alice.New(
		recoveryMiddleware,
		loggingMiddleware,
		cors.AllowAll().Handler,
	)

	log.Println("RESTAPI: Binding to", bindAddr)
	err := http.ListenAndServe(bindAddr, chain.Then(a.router))
	if err != nil {
		log.Fatal(err)
	}
}

func (a *restAPI) setupRouter() {
	r := mux.NewRouter()

	r.HandleFunc("/", a.index).Methods("GET")
	// targets
	r.HandleFunc("/targets", a.getTargets).Methods("GET")
	r.HandleFunc("/targets/{id}", a.getTarget).Methods("GET")
	r.HandleFunc("/targets/{id}/logs", a.requestTargetLogs).Methods("PUT")
	// tasks
	r.HandleFunc("/orders", a.getOrders).Methods("GET")
	r.HandleFunc("/orders/{id}", a.getOrder).Methods("GET")
	r.HandleFunc("/orders", a.addOrder).Methods("POST")
	// logs
	r.HandleFunc("/logs", a.getLogs).Methods("GET")
	r.HandleFunc("/logs/search", a.searchLogs).Methods("GET")

	// static
	ui := http.Dir(WorkDir + "/ui")
	r.PathPrefix("/ui").Handler(http.StripPrefix("/ui", http.FileServer(ui)))

	// websocket
	r.PathPrefix("/events").HandlerFunc(a.websocket)

	a.router = r
}

func (a *restAPI) index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome!\n")
}

func (a *restAPI) addOrder(w http.ResponseWriter, r *http.Request) {

	decoder := yaml.NewDecoder(r.Body)
	defer r.Body.Close()

	var order storage.Order
	err := decoder.Decode(&order)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	log.Println("Received order:", order)

	err = order.Validate()
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, "Invalid order: ", err)
		return
	}

	err = a.manager.addOrder(&order)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	// order is accepted
	b, err := json.Marshal(order) // marshal updated order
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusAccepted, b)
	return
}

func (a *restAPI) getOrder(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	order, err := a.manager.getOrder(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if order == nil {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	b, err := json.Marshal(order)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) getOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, perPage, err := parsePagingAttributes(query)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	orders, total, err := a.manager.getOrders(page, perPage)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&list{total, orders, page, perPage})
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) getTargets(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	tags := query.Get(_tags)

	page, perPage, err := parsePagingAttributes(query)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	var tagSlice []string
	if tags != "" {
		tagSlice = strings.Split(tags, ",")
	}

	targets, total, err := a.manager.getTargets(tagSlice, page, perPage)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&list{total, targets, page, perPage})
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) getTarget(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	target, err := a.manager.getTarget(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if target == nil {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	b, err := json.Marshal(target)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) requestTargetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	target, err := a.manager.getTarget(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if target == nil {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	err = a.manager.requestLogs(id)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}
	HTTPResponseSuccess(w, http.StatusOK, "Requested logs for ", id)
	return
}

func (a *restAPI) getLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	fields := map[string]string{
		_target:  query.Get(_target),
		_task:    query.Get(_task),
		_stage:   query.Get(_stage),
		_command: query.Get(_command),
		_time:    "",
	}

	page, perPage, err := parsePagingAttributes(query)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	sortBy, ascending, err := parseSortingParameters(query)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}
	if sortBy == "" {
		sortBy = _time
	}
	if _, ok := fields[sortBy]; !ok {
		HTTPResponseError(w, http.StatusBadRequest, fmt.Sprintf("%s query parameter has invalid value", _sortBy))
		return
	}

	logs, total, err := a.manager.getLogs(fields[_target], fields[_task], fields[_stage], fields[_command], sortBy, ascending, page, perPage)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	b, err := json.Marshal(&list{total, logs, page, perPage})
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) searchLogs(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	search := make(map[string]interface{})
	// body is either empty or a search object
	if len(body) > 0 {
		err = json.Unmarshal(body, &search)
		if err != nil {
			HTTPResponseError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	logs, total, err := a.manager.searchLogs(search)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	b, err := json.Marshal(&list{Total: total, Items: logs}) // without paging info
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) websocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{} // use default options
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("websocket: upgrade error:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer c.Close()

	query := r.URL.Query()
	topics := []string{EventLogs, EventTargetAdded, EventTargetUpdated}
	if topicsQuery := query.Get(_topics); topicsQuery != "" {
		topics = strings.Split(topicsQuery, ",")
	}

	events := a.manager.events.Sub(topics...)
	log.Println("websocket: client subscribed to:", topics)

	for event := range events {
		log.Println("websocket: sending update!")
		b, _ := json.Marshal(event)
		err = c.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("websocket: write error:", err)
			go a.manager.events.Unsub(events)
			if len(events) > 0 {
				<-events
			}
			log.Println("websocket: closed the subscriber.")
			break
		}
	}

}

func parsePagingAttributes(query url.Values) (page int, perPage int, err error) {
	page, perPage = defaultPage, defaultPerPage
	if query.Get(_page) != "" {
		page, err = strconv.Atoi(query.Get(_page))
		if err != nil {
			return 0, 0, fmt.Errorf("error parsing %s query parameter: %s", _page, err)
		}
		if page < 1 {
			return 0, 0, fmt.Errorf("%s query parameter must be positive", _page)
		}
	}
	if query.Get(_perPage) != "" {
		perPage, err = strconv.Atoi(query.Get(_perPage))
		if err != nil {
			return 0, 0, fmt.Errorf("error parsing %s query parameter: %s", _perPage, err)
		}
		if perPage < 1 {
			return 0, 0, fmt.Errorf("%s query parameter must be positive", _perPage)
		}
	}
	return page, perPage, nil
}

func parseSortingParameters(query url.Values) (sortBy string, ascending bool, err error) {
	sortBy, order := query.Get(_sortBy), query.Get(_sortOrder)
	if order == "" || order == _asc {
		return sortBy, true, nil
	} else if order == _desc {
		return sortBy, false, nil
	}
	return "", false, fmt.Errorf("%s query parameter has invalid value", _sortOrder)
}

// HTTPResponseError serializes and writes an error response
//	If no message is provided, the status text will be set as the message
func HTTPResponseError(w http.ResponseWriter, code int, message ...interface{}) {
	if len(message) == 0 {
		message = make([]interface{}, 1)
		message[0] = http.StatusText(code)
	}
	log.Println("Request error:", message)
	body, _ := json.Marshal(&map[string]string{
		"error": fmt.Sprint(message...),
	})
	HTTPResponse(w, code, body)
}

// HTTPResponseSuccess serializes and writes a success response
//	If no message is provided, the status text will be set as the message
func HTTPResponseSuccess(w http.ResponseWriter, code int, message ...interface{}) {
	if len(message) == 0 {
		message = make([]interface{}, 1)
		message[0] = http.StatusText(code)
	}
	body, _ := json.Marshal(&map[string]string{
		"message": fmt.Sprint(message...),
	})
	HTTPResponse(w, code, body)
}

// HTTPResponse writes a response
func HTTPResponse(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write(body)
	if err != nil {
		log.Printf("HTTPResponse: error writing reponse: %s", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
		log.Println(r.Method, r.URL, "responded in", time.Since(start))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC: %v\n%s", r, debug.Stack())
				HTTPResponseError(w, 500, r)
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

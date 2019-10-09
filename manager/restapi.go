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

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/storage"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/justinas/alice"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
	"gopkg.in/yaml.v2"
)

const (
	// query parameter key/values
	_page            = "page"
	_perPage         = "perPage"
	_sortBy          = "sortBy"
	_sortOrder       = "sortOrder"
	_asc             = "asc"
	_desc            = "desc"
	_time            = "time"
	_target          = "target"
	_task            = "task"
	_stage           = "stage"
	_command         = "command"
	_output          = "output"
	_error           = "error"
	_tags            = "tags"
	_tag             = "tag"
	_total           = "total"
	_topics          = "topics"
	_name            = "name"
	_description     = "description"
	_tokenHeader     = "X-Auth-Token"
	defaultPage      = 1
	defaultPerPage   = 100
	defaultSortOrder = _asc
)

type restAPI struct {
	manager *manager
	router  *mux.Router
}

type list struct {
	Total   int64       `json:"total"`
	Items   interface{} `json:"items"` // array of anything
	Page    int         `json:"page"`
	PerPage int         `json:"perPage"`
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

	r.HandleFunc("/", a.index).Methods(http.MethodGet)
	// targets
	r.HandleFunc("/targets", a.getTargets).Methods(http.MethodGet)
	r.HandleFunc("/targets/{id}", a.getTarget).Methods(http.MethodGet)
	r.HandleFunc("/targets/{id}", a.deleteTarget).Methods(http.MethodDelete)
	r.HandleFunc("/targets/{id}", a.updateTarget).Methods(http.MethodPut)
	r.HandleFunc("/targets/{id}/stop", a.stopTargetOrders).Methods(http.MethodPut)
	r.HandleFunc("/targets/{id}/logs", a.requestTargetLogs).Methods(http.MethodPut)
	r.HandleFunc("/targets/{id}/command", a.executeCommand).Methods(http.MethodPut)
	r.HandleFunc("/targets/{id}/command", a.stopCommand).Methods(http.MethodDelete)
	// tasks
	r.HandleFunc("/orders", a.getOrders).Methods(http.MethodGet)
	r.HandleFunc("/orders/{id}", a.getOrder).Methods(http.MethodGet)
	r.HandleFunc("/orders/{id}", a.deleteOrder).Methods(http.MethodDelete)
	r.HandleFunc("/orders/{id}/stop", a.stopOrder).Methods(http.MethodPut)
	r.HandleFunc("/orders/{id}/status", a.getOrderStatus).Methods(http.MethodGet)
	r.HandleFunc("/orders", a.addOrder).Methods(http.MethodPost)
	// logs
	r.HandleFunc("/logs", a.getLogs).Methods(http.MethodGet)
	r.HandleFunc("/logs/search", a.searchLogs).Methods(http.MethodGet)
	// tokens
	r.HandleFunc("/token_sets", a.getTokenSets).Methods(http.MethodGet)
	r.HandleFunc("/token_sets", a.createTokenSet).Methods(http.MethodPost)
	r.HandleFunc("/token_sets/{name}", a.getTokenSet).Methods(http.MethodGet)
	r.HandleFunc("/token_sets/{name}", a.deleteTokenSet).Methods(http.MethodDelete)
	r.HandleFunc("/rpc/targets", a.registerTarget).Methods(http.MethodPost)
	r.HandleFunc("/rpc/server_info", a.getServerInfo).Methods(http.MethodGet)
	// health
	r.HandleFunc("/health", a.getHealth).Methods(http.MethodGet)

	// static
	ui := http.Dir(WorkDir + "/ui")
	r.PathPrefix("/ui").Handler(http.StripPrefix("/ui", http.FileServer(ui)))

	// websocket
	r.PathPrefix("/events").HandlerFunc(a.websocket)

	a.router = r
}

func (a *restAPI) index(w http.ResponseWriter, r *http.Request) {
	info, err := a.manager.getServerInfo()
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&info)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusOK, b)
}

func (a *restAPI) addOrder(w http.ResponseWriter, r *http.Request) {

	decoder := yaml.NewDecoder(r.Body)
	defer r.Body.Close()

	var order storage.Order
	err := decoder.Decode(&order)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err)
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

	b, err := json.Marshal(order) // marshal updated order
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusCreated, b)
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

func (a *restAPI) deleteOrder(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	found, err := a.manager.deleteOrder(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	return
}

func (a *restAPI) stopTargetOrders(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	found, err := a.manager.stopTargetOrders(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	HTTPResponseSuccess(w, http.StatusOK, "Sent stop signal to ", id)
	return
}

func (a *restAPI) stopOrder(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	found, list, err := a.manager.stopOrder(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	HTTPResponseSuccess(w, http.StatusOK, "Sent stop signal to ", list)
	return
}

func (a *restAPI) getOrderStatus(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	found, targets, err := a.manager.getOrderStatus(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	b, err := json.Marshal(targets)
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

	_, ascending, err := parseSortingParameters(query)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	descr := query.Get(_description)

	orders, total, err := a.manager.getOrders(descr, ascending, page, perPage)
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

func (a *restAPI) deleteTarget(w http.ResponseWriter, r *http.Request) {

	id := mux.Vars(r)["id"]

	found, err := a.manager.deleteTarget(id)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	return
}

func (a *restAPI) registerTarget(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var target storage.Target
	err := decoder.Decode(&target)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, "error parsing body: "+err.Error())
		return
	}

	token := r.Header.Get(_tokenHeader)
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	authorized, conflict, err := a.manager.registerTarget(&target, token)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	} else if !authorized {
		w.WriteHeader(http.StatusUnauthorized)
		return
	} else if conflict {
		w.WriteHeader(http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	return
}

func (a *restAPI) getServerInfo(w http.ResponseWriter, r *http.Request) {

	info, err := a.manager.getServerInfo()
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&info)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) updateTarget(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var target storage.Target
	err := decoder.Decode(&target)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err)
		return
	}

	id := mux.Vars(r)["id"]
	found, err := a.manager.updateTarget(id, &target)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		HTTPResponseError(w, http.StatusNotFound, id+" is not found!")
		return
	}

	w.WriteHeader(http.StatusOK)
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

func (a *restAPI) executeCommand(w http.ResponseWriter, r *http.Request) {
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

	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err)
		return
	}

	body := make(map[string]string)
	err = json.Unmarshal(b, &body)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err)
		return
	}

	if body["command"] == "" {
		HTTPResponseError(w, http.StatusBadRequest, "command not given")
		return
	}

	err = a.manager.terminalCommand(id, body["command"])
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponseSuccess(w, http.StatusOK, "Sent command to ", id)
	return
}

func (a *restAPI) stopCommand(w http.ResponseWriter, r *http.Request) {
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

	err = a.manager.terminalCommand(id, model.TerminalStop)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	return
}

func (a *restAPI) getLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	fields := map[string]string{
		_target:  query.Get(_target),
		_task:    query.Get(_task),
		_stage:   query.Get(_stage),
		_command: query.Get(_command),
		_output:  query.Get(_output),
		_error:   query.Get(_error),
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

	logs, total, err := a.manager.getLogs(
		fields[_target],
		fields[_task],
		fields[_stage],
		fields[_command],
		fields[_output],
		fields[_error],
		sortBy, ascending, page, perPage)
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	if r.Header.Get("Accept") == "text/plain" {
		_, err = w.Write([]byte(fmt.Sprintf("page: %d, perPage: %d, total: %d\n", page, perPage, total)))
		if err != nil {
			HTTPResponseError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, l := range logs {
			_, err = w.Write([]byte(fmt.Sprintln(l.Time, l.Target, l.Task, l.Stage, l.Command, l.Output, l.Error)))
			if err != nil {
				HTTPResponseError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
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

	b, err := json.Marshal(&list{total, logs, 1, int(total)})
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) getTokenSets(w http.ResponseWriter, r *http.Request) {

	sets, err := a.manager.getTokenSets()
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&list{int64(len(sets)), sets, 1, len(sets)})
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) createTokenSet(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if query.Get(_total) == "" {
		HTTPResponseError(w, http.StatusBadRequest, _total+" parameter is not given")
		return
	}
	if query.Get(_name) == "" {
		HTTPResponseError(w, http.StatusBadRequest, _name+" parameter is not given")
		return
	}

	total, err := strconv.Atoi(query.Get(_total))
	if err != nil {
		HTTPResponseError(w, http.StatusBadRequest, fmt.Sprintf("error parsing %s: %s", _total, err))
		return
	}

	tokenSet, conflict, err := a.manager.createTokenSet(total, query.Get(_name))
	if err != nil {
		log.Printf("Error creating set: %s", err)
		HTTPResponseError(w, http.StatusInternalServerError, "error creating set") // should be vague
		return
	}
	if conflict {
		HTTPResponseError(w, http.StatusConflict, fmt.Sprintf("name is not unique: %s", query.Get(_name)))
		return
	}

	b, err := json.Marshal(tokenSet)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	HTTPResponse(w, http.StatusCreated, b)
	return

}

func (a *restAPI) getTokenSet(w http.ResponseWriter, r *http.Request) {
	tokenSet, err := a.manager.getTokenSet(mux.Vars(r)[_name])
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, "error getting set: "+err.Error())
		return
	}

	b, err := json.Marshal(tokenSet)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) deleteTokenSet(w http.ResponseWriter, r *http.Request) {
	err := a.manager.deleteTokenSet(mux.Vars(r)[_name])
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, "error deleting set: "+err.Error())
		return
	}

	return
}

func (a *restAPI) getHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK!"))
}

func (a *restAPI) websocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }, // allow all origins
	}
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
	task := query.Get(_task)
	target := query.Get(_target)

	events := a.manager.events.Sub(topics...)
	defer a.manager.events.Unsub(events) // publisher should only use the TryPub method to avoid panics

	for raw := range events {
		// filter logs, very inefficiently!
		if e, ok := raw.(event); ok {
			if e.Topic == EventLogs {
				if logs, ok := e.Payload.([]storage.Log); ok {
					filtered := make([]storage.Log, 0, len(logs))
					for i := range logs {
						if (task == "" || logs[i].Task == task) && (target == "" || logs[i].Target == target) {
							filtered = append(filtered, logs[i])
						}
					}
					if len(filtered) == 0 { // nothing left to send
						continue
					}
					raw = event{Topic: e.Topic, Payload: filtered}
				}
			}
		}
		b, _ := json.Marshal(raw)
		err = c.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("websocket: write error:", err)
			break
		}
		log.Println("websocket: sent event.")
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
	if order == "" {
		order = defaultSortOrder
	}
	if order == _asc {
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
		nw := negroni.NewResponseWriter(w)
		next.ServeHTTP(nw, r)
		log.Printf("\"%s %s %s\" %d %d %v\n", r.Method, r.URL.String(), r.Proto, nw.Status(), nw.Size(), time.Since(start))
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

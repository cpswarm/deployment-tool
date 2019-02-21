package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/storage"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	yaml "gopkg.in/yaml.v2"
)

type restAPI struct {
	manager *manager
	router  *mux.Router
}

type searchResults struct {
	Total int64       `json:"total"`
	Hits  interface{} `json:"hits"` // array of anything
}

func startRESTAPI(bindAddr string, manager *manager) {

	a := restAPI{
		manager: manager,
	}

	a.setupRouter()

	log.Println("RESTAPI: Binding to", bindAddr)
	err := http.ListenAndServe(bindAddr, a.router)
	if err != nil {
		log.Fatal(err)
	}
}

func (a *restAPI) setupRouter() {
	r := mux.NewRouter()
	// targets
	r.HandleFunc("/targets", a.getTargets).Methods("GET")
	r.HandleFunc("/targets/{id}", a.getTarget).Methods("GET")
	r.HandleFunc("/targets/{id}/logs", a.requestTargetLogs).Methods("PUT")
	// tasks
	r.HandleFunc("/orders", a.getOrders).Methods("GET")
	r.HandleFunc("/orders/{id}", a.getOrder).Methods("GET")
	r.HandleFunc("/orders", a.addOrder).Methods("POST")
	// logs
	r.HandleFunc("/logs/search", a.searchLogs).Methods("GET")
	// tokens
	//r.HandleFunc("/targets/{total}", a.createTokens).Methods("PUT")

	/*
		/orders
		/targets
		/logs?targetID=id&task=id
		/tokens (post # or get)
		/users (future work)

		Events:
		new order
		new/updated target
		new log
	*/

	// static
	ui := http.Dir(WorkDir + "/ui")
	r.PathPrefix("/ui").Handler(http.StripPrefix("/ui", http.FileServer(ui)))
	r.PathPrefix("/ws").HandlerFunc(a.websocket)

	r.Use(loggingMiddleware)

	a.router = r
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
		log.Println(r.Method, r.URL, "responded in", time.Since(start))
	})
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

	orders, total, err := a.manager.getOrders()
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&searchResults{total, orders})
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) getTargets(w http.ResponseWriter, r *http.Request) {

	targets, total, err := a.manager.getTargets()
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, err := json.Marshal(&searchResults{total, targets})
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

	b, err := json.Marshal(&searchResults{Total: total, Hits: logs})
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
		return
	}
	defer c.Close()
	for {
		a.manager.update.L.Lock()
		a.manager.update.Wait()
		log.Println("websocket: sending update!")

		targets, _, err := a.manager.getTargets()
		if err != nil {
			log.Println("websocket: error getting targets:", err)
			a.manager.update.L.Unlock()
			break
		}

		b, _ := json.Marshal(targets)
		err = c.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("websocket: write error:", err)
			a.manager.update.L.Unlock()
			break
		}
		a.manager.update.L.Unlock()
	}
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

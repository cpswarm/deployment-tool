package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"code.linksmart.eu/dt/deployment-tool/model"
	"gopkg.in/yaml.v2"
)

type restAPI struct {
	manager *manager
}

func startRESTAPI(bindAddr string, manager *manager) {

	a := restAPI{
		manager: manager,
	}

	h := http.NewServeMux()

	h.HandleFunc("/tasks", a.TaskHandler)
	h.HandleFunc("/targets", a.TargetHandler)

	log.Println("RESTAPI: Binding to", bindAddr)
	err := http.ListenAndServe(bindAddr, h)
	if err != nil {
		log.Fatal(err)
	}
}

func (a *restAPI) TaskHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)
	defer log.Println(r.Method, r.URL, "responded.")

	switch r.Method {
	case http.MethodPost:
		a.AddTask(w, r)
		return
	case http.MethodGet:
		a.ListTasks(w, r)
		return
	default:
		HTTPResponseError(w, http.StatusMethodNotAllowed)
		return
	}

}

func (a *restAPI) AddTask(w http.ResponseWriter, r *http.Request) {

	decoder := yaml.NewDecoder(r.Body)
	defer r.Body.Close()

	var descr TaskDescription
	err := decoder.Decode(&descr)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}
	log.Println("Received task descr:", descr)

	createdDescr, err := a.manager.addTaskDescr(descr)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	b, _ := json.Marshal(createdDescr)

	// task is only accepted, but may not succeed
	HTTPResponse(w, http.StatusAccepted, b)
	return
}

func (a *restAPI) ListTasks(w http.ResponseWriter, r *http.Request) {

	b, err := json.Marshal(a.manager.taskDescriptions)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

func (a *restAPI) TargetHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)
	defer log.Println(r.Method, r.URL, "responded.")

	switch r.Method {
	case http.MethodGet:
		a.ListTargets(w, r)
		return
	default:
		HTTPResponseError(w, http.StatusMethodNotAllowed)
		return
	}

}

func (a *restAPI) ListTargets(w http.ResponseWriter, r *http.Request) {

	var targets []*model.Target
	for _, t := range a.manager.targets {
		targets = append(targets, t)
	}
	// TODO sort by ID

	b, err := json.Marshal(targets)
	if err != nil {
		HTTPResponseError(w, http.StatusInternalServerError, err)
		return
	}

	HTTPResponse(w, http.StatusOK, b)
	return
}

// HTTPResponseError serializes and writes an error response
//	If no message is provided, the status text will be set as the message
func HTTPResponseError(w http.ResponseWriter, code int, message ...interface{}) {
	if len(message) == 0 {
		message = make([]interface{}, 1)
		message[0] = http.StatusText(code)
	}
	body, _ := json.Marshal(&map[string]string{
		"error": fmt.Sprint(message...),
	})
	HTTPResponse(w, code, body)
}

// HTTPResponse writes a response
func HTTPResponse(w http.ResponseWriter, code int, body []byte) {
	w.WriteHeader(code)
	_, err := w.Write(body)
	if err != nil {
		log.Printf("HTTPResponse: error writing reponse: %s", err)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	yaml "gopkg.in/yaml.v2"
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
	case http.MethodPut:
		a.AddTask(w, r)
		return
	case http.MethodGet:
		a.ListTasks(w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

}

func (a *restAPI) AddTask(w http.ResponseWriter, r *http.Request) {

	decoder := yaml.NewDecoder(r.Body)
	defer r.Body.Close()

	var descr TaskDescription
	err := decoder.Decode(&descr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
		return
	}
	log.Println("Received task descr:", descr)

	id, err := a.manager.addTaskDescr(descr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
		return
	}

	// task is only accepted, but may not succeed
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, http.StatusText(http.StatusAccepted), id)
}

func (a *restAPI) ListTasks(w http.ResponseWriter, r *http.Request) {

	b, err := json.Marshal(a.manager.taskDescriptions)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
		return
	}

	w.Write(b)
}

func (a *restAPI) TargetHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)
	defer log.Println(r.Method, r.URL, "responded.")

	switch r.Method {
	case http.MethodGet:
		a.ListTargets(w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

}

func (a *restAPI) ListTargets(w http.ResponseWriter, r *http.Request) {

	b, err := json.Marshal(a.manager.targets)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
		return
	}

	w.Write(b)
}

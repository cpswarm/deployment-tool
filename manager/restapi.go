package main

import (
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

	http.HandleFunc("/tasks", a.Tasks)

	log.Println("RESTAPI: Binding to", bindAddr)
	err := http.ListenAndServe(bindAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func (a *restAPI) Tasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

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

	go a.manager.sendTask(descr)
	// task is only accepted, but may not succeed
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, http.StatusText(http.StatusAccepted))
}

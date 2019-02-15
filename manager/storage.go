package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"github.com/olivere/elastic"
)

type Storage interface {
	GetOrders() ([]order, error) // TODO add pagination
	AddOrder(*order) error
	GetOrder(string) (*order, error)
	//DeleteOrder(string) error
	//
	GetTargets() ([]model.Target, error) // TODO add pagination
	AddTarget(*model.Target) error
	GetTarget(string) (*model.Target, error)
	//DeleteTarget(string) error
	//
	GetLogs(target, task string) ([]model.Log, error)
	AddLog(*model.Log) error
	//DeleteLogs(target string) error
}

type storage struct {
	client *elastic.Client
	ctx    context.Context
}

const (
	indexTarget   = "target"
	mappingTarget = `{
    	"settings" : {
        	"number_of_shards" : 1,
			"number_of_replicas": 0,
			"refresh_interval": "1s"
    	},
    	"mappings" : {
	        "_doc" : {
				"dynamic": "strict",
	            "properties" : {
					"id": { "type" : "keyword" },
	            	"tags": { "type" : "keyword" },
	            	"createdAt": {"type": "date"},
					"updatedAt": {"type": "date"},
					"taskID": { "type" : "keyword" }
"
	            }
    	    }
	    }
	}`
	indexOrder   = "order"
	mappingOrder = `{
	    "settings" : {
	        "number_of_shards" : 1,
			"number_of_replicas": 0,
			"refresh_interval": "1s"
	    },
	    "mappings" : {
	        "_doc" : {
				"dynamic": "strict",
	            "properties" : {
					"id": { "type" : "keyword" },
					"debug": {"type": "boolean"},
	            	"createdAt": {"type": "date"},
	            	"buildType": {"type": "keyword"}
	            }
	        }
	    }
	}`
	indexLog   = "log"
	mappingLog = `{
	    "settings" : {
	        "number_of_shards" : 1,
			"number_of_replicas": 0,
			"refresh_interval": "1s"
	    },
	    "mappings" : {
	        "_doc" : {
				"dynamic": "strict",
	            "properties" : {
	            	"time": { "type" : "date" },
	            	"target": {"type": "keyword"},
	            	"task": {"type": "keyword"},
	            	"command": {"type": "keyword"},
	            	"output": {"type": "text"},
	            	"error": {"type": "boolean"}
	            }
	        }
	    }
	}`
	typeFixed = "_doc"
)

func NewElasticStorage(url string) (Storage, error) {
	ctx := context.Background()

	client, err := elastic.NewSimpleClient(
		elastic.SetURL(url),
		//elastic.SetTraceLog(log.New(os.Stdout, "[Elastic Search] ", 0)),
	)
	if err != nil {
		return nil, err
	}

	// Ping the Elasticsearch server to get e.g. the version number
	info, code, err := client.Ping(url).Do(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("Elasticsearch returned with code %d and version %s", code, info.Version.Number)

	s := storage{
		ctx:    ctx,
		client: client,
	}

	// create indices
	err = s.createIndex(indexTarget, mappingTarget)
	if err != nil {
		return nil, err
	}
	err = s.createIndex(indexOrder, mappingOrder)
	if err != nil {
		return nil, err
	}
	err = s.createIndex(indexLog, mappingLog)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *storage) createIndex(index, mapping string) error {
	// Use the IndexExists service to check if a specified index exists.
	exists, err := s.client.IndexExists(index).Do(s.ctx)
	if err != nil {
		return fmt.Errorf("error checking index: %s", err)
	}
	if !exists {
		// Create a new index.
		createIndex, err := s.client.CreateIndex(index).
			BodyString(mapping).Do(s.ctx)
		if err != nil {
			return fmt.Errorf("error creating index %s: %s", index, err)
		}
		if !createIndex.Acknowledged {
			// Not acknowledged
			log.Printf("Did not acknowledge creation of index: %s", index)
		}
		log.Printf("Created index: %s", index)
	}
	return nil
}

func (s *storage) AddTarget(target *model.Target) error {
	res, err := s.client.Index().Index(indexTarget).Type(typeFixed).
		Id(target.ID).BodyJson(target).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s/%s v%d", res.Index, res.Id, res.Version)
	return nil
}

func (s *storage) GetTargets() ([]model.Target, error) {
	searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
		Sort("id", true).From(0).Size(100).Do(s.ctx)
	if err != nil {
		return nil, err
	}

	var targets []model.Target
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var target model.Target
			err := json.Unmarshal(*hit.Source, &target)
			if err != nil {
				return nil, err
			}
			targets = append(targets, target)
		}
	} else {
		log.Print("Found no entries")
	}
	return targets, nil
}

func (s *storage) GetTarget(id string) (*model.Target, error) {
	res, err := s.client.Get().Index(indexTarget).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		return nil, err
	}
	if res.Found {
		log.Printf("Got document %s/%s v%d", res.Index, res.Id, res.Version)
	}
	var target model.Target
	err = json.Unmarshal(*res.Source, &target)
	if err != nil {
		return nil, err
	}
	return &target, nil
}

func (s *storage) AddOrder(order *order) error {
	res, err := s.client.Index().Index(indexOrder).Type(typeFixed).
		Id(order.ID).BodyJson(order).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s/%s v%d", res.Index, res.Id, res.Version)
	return nil
}

func (s *storage) GetOrders() ([]order, error) {
	searchResult, err := s.client.Search().Index(indexOrder).Type(typeFixed).
		Sort("id", true).From(0).Size(100).Do(s.ctx)
	if err != nil {
		return nil, err
	}

	var orders []order
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var order order
			err := json.Unmarshal(*hit.Source, &order)
			if err != nil {
				return nil, err
			}
			orders = append(orders, order)
		}
	} else {
		log.Print("Found no entries")
	}
	return orders, nil
}

func (s *storage) GetOrder(id string) (*order, error) {
	res, err := s.client.Get().Index(indexLog).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		return nil, err
	}
	if res.Found {
		log.Printf("Got document %s/%s v%d", res.Index, res.Id, res.Version)
	}
	var order order
	err = json.Unmarshal(*res.Source, &order)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *storage) GetLogs(target, task string) ([]model.Log, error) {

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchQuery("target", target)).
		Must(elastic.NewMatchQuery("task", task))

	searchResult, err := s.client.Search().Index(indexOrder).Type(typeFixed).
		Sort("time", true).From(0).Size(100).Query(query).Do(s.ctx)
	if err != nil {
		return nil, err
	}

	var logs []model.Log
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var l model.Log
			err := json.Unmarshal(*hit.Source, &l)
			if err != nil {
				return nil, err
			}
			logs = append(logs, l)
		}
	} else {
		log.Print("Found no entries")
	}
	return logs, nil
}

func (s *storage) AddLog(logM *model.Log) error {
	res, err := s.client.Index().Index(indexLog).Type(typeFixed).
		BodyJson(logM).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s/%s", res.Index, res.Id)
	return nil
}

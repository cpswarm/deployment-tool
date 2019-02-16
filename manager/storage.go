package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"github.com/olivere/elastic"
)

type Storage interface {
	GetOrders() ([]order, int64, error) // TODO add search and pagination
	AddOrder(*order) error
	GetOrder(string) (*order, error)
	//DeleteOrder(string) error
	//
	GetTargets() ([]model.Target, int64, error) // TODO add search and pagination
	AddTarget(*model.Target) error
	GetTarget(string) (*model.Target, error)
	//DeleteTarget(string) error
	//
	GetLogs(map[string]interface{}) ([]model.LogStored, int64, error)
	AddLog(*model.LogStored) error
	//DeleteLogs(target string) error
}

type storage struct {
	client *elastic.Client
	ctx    context.Context
}

type mapping struct {
	Settings struct {
		Shards          int    `json:"number_of_shards"`
		Replicas        int    `json:"number_of_replicas"`
		RefreshInterval string `json:"refresh_interval"`
	} `json:"settings"`
	Mappings struct {
		Doc struct {
			Dynamic string                 `json:"dynamic"`
			Prop    map[string]mappingProp `json:"properties"`
		} `json:"_doc"`
	} `json:"mappings"`
}

type mappingProp struct {
	Type string `json:"type"`
}

const (
	indexTarget     = "target"
	indexOrder      = "order"
	indexLog        = "log"
	typeFixed       = "_doc"
	mappingStrict   = "strict"
	propTypeKeyword = "keyword"
	propTypeDate    = "date"
	propTypeText    = "text"
	propTypeBool    = "boolean"
)

func NewElasticStorage(url string) (Storage, error) {
	ctx := context.Background()

	client, err := elastic.NewSimpleClient(
		elastic.SetURL(url),
		elastic.SetTraceLog(log.New(os.Stdout, "[Elastic Search] ", 0)),
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
	m := mapping{}
	m.Settings.Shards = 1
	m.Settings.Replicas = 0
	m.Settings.RefreshInterval = "1s"
	m.Mappings.Doc.Dynamic = mappingStrict
	m.Mappings.Doc.Prop = map[string]mappingProp{
		"id":        {Type: propTypeKeyword},
		"tags":      {Type: propTypeKeyword},
		"createdAt": {Type: propTypeDate},
		"updatedAt": {Type: propTypeDate},
		"type":      {Type: propTypeKeyword},
	}
	err = s.createIndex(indexTarget, m)
	if err != nil {
		return nil, err
	}

	m = mapping{}
	m.Settings.Shards = 1
	m.Settings.Replicas = 0
	m.Settings.RefreshInterval = "1s"
	m.Mappings.Doc.Dynamic = mappingStrict
	m.Mappings.Doc.Prop = map[string]mappingProp{
		"id":        {Type: propTypeKeyword},
		"debug":     {Type: propTypeBool},
		"createdAt": {Type: propTypeDate},
	}
	err = s.createIndex(indexOrder, m)
	if err != nil {
		return nil, err
	}

	m = mapping{}
	m.Settings.Shards = 1
	m.Settings.Replicas = 0
	m.Settings.RefreshInterval = "1s"
	m.Mappings.Doc.Dynamic = mappingStrict
	m.Mappings.Doc.Prop = map[string]mappingProp{
		"time":    {Type: propTypeDate},
		"target":  {Type: propTypeKeyword},
		"task":    {Type: propTypeKeyword},
		"stage":   {Type: propTypeKeyword},
		"command": {Type: propTypeKeyword},
		"output":  {Type: propTypeText},
		"error":   {Type: propTypeBool},
	}
	err = s.createIndex(indexLog, m)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *storage) createIndex(index string, mapping mapping) error {
	// Use the IndexExists service to check if a specified index exists.
	exists, err := s.client.IndexExists(index).Do(s.ctx)
	if err != nil {
		return fmt.Errorf("error checking index: %s", err)
	}
	if !exists {
		// Create a new index.
		createIndex, err := s.client.CreateIndex(index).
			BodyJson(mapping).Do(s.ctx)
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

// TODO add query and search
func (s *storage) GetTargets() ([]model.Target, int64, error) {
	searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
		Sort("id", true).From(0).Size(100).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var targets []model.Target
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var target model.Target
			err := json.Unmarshal(*hit.Source, &target)
			if err != nil {
				return nil, 0, err
			}
			targets = append(targets, target)
		}
	} else {
		log.Print("Found no entries")
	}
	return targets, searchResult.Hits.TotalHits, nil
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

// TODO add query and search
func (s *storage) GetOrders() ([]order, int64, error) {
	searchResult, err := s.client.Search().Index(indexOrder).Type(typeFixed).
		Sort("id", true).From(0).Size(100).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var orders []order
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var order order
			err := json.Unmarshal(*hit.Source, &order)
			if err != nil {
				return nil, 0, err
			}
			orders = append(orders, order)
		}
	} else {
		log.Print("Found no entries")
	}
	return orders, searchResult.Hits.TotalHits, nil
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

func (s *storage) GetLogs(source map[string]interface{}) ([]model.LogStored, int64, error) {

	//query := elastic.NewBoolQuery().
	//	Must(elastic.NewMatchQuery("target", target)).
	//	Must(elastic.NewMatchQuery("task", task))
	//
	//searchResult, err := s.client.Search().Index(indexOrder).Type(typeFixed).
	//	Sort("time", true).From(0).Size(100).Query(query).Do(s.ctx)
	//if err != nil {
	//	return nil, err
	//}

	searchResult, err := s.client.Search().Index(indexLog).Type(typeFixed).
		Source(source).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var logs []model.LogStored
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)
		for _, hit := range searchResult.Hits.Hits {
			var l model.LogStored
			err := json.Unmarshal(*hit.Source, &l)
			if err != nil {
				return nil, 0, err
			}
			logs = append(logs, l)
		}
	} else {
		log.Print("Found no entries")
	}
	return logs, searchResult.Hits.TotalHits, nil
}

func (s *storage) AddLog(logM *model.LogStored) error {
	res, err := s.client.Index().Index(indexLog).Type(typeFixed).
		BodyJson(logM).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s/%s", res.Index, res.Id)
	return nil
}

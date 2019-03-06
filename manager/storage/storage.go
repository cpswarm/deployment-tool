package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"github.com/olivere/elastic"
)

type Storage interface {
	GetOrders(from, size int) ([]Order, int64, error)
	AddOrder(*Order) error
	GetOrder(string) (*Order, error)
	//DeleteOrder(string) error
	//
	GetTargets(tags []string, from, size int) ([]Target, int64, error)
	PatchTarget(id string, target *Target) (bool, error)
	MatchTargets(ids, tags []string) (allIDs, hitIDs, hitTags []string, err error)
	SearchTargets(map[string]interface{}) ([]Target, int64, error)
	AddTarget(*Target) error
	GetTarget(string) (*Target, error)
	//DeleteTarget(string) error
	//
	GetLogs(target, task, stage, command, output, error, sortField string, sortAsc bool, from, size int) ([]Log, int64, error)
	SearchLogs(map[string]interface{}) ([]Log, int64, error)
	AddLog(*Log) error
	AddLogs(logs []Log) error
	//DeleteLogs(target string) error
	DeliveredTask(target, task string) (bool, error)
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
	Type       string                 `json:"type,omitempty"`
	Properties map[string]mappingProp `json:"properties,omitempty"` // object datatype
}

const (
	envElasticDebug = "DEBUG_ELASTIC"
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
	log.Println("Elasticsearch URL:", url)
	ctx := context.Background()

	opts := []elastic.ClientOptionFunc{elastic.SetURL(url)}

	if os.Getenv(envElasticDebug) == "1" {
		opts = append(opts, elastic.SetTraceLog(log.New(os.Stdout, "[Elastic Debug] ", 0)))
	}

	client, err := elastic.NewSimpleClient(opts...)
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
	var m mapping
	m.Settings.Shards = 1
	m.Settings.Replicas = 0
	m.Settings.RefreshInterval = "1s"
	m.Mappings.Doc.Dynamic = mappingStrict
	m.Mappings.Doc.Prop = map[string]mappingProp{
		"id":           {Type: propTypeKeyword},
		"tags":         {Type: propTypeKeyword}, // array
		"updatedAt":    {Type: propTypeDate},
		"logRequestAt": {Type: propTypeDate},
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
		"build": {
			Properties: map[string]mappingProp{
				"commands":  {Type: propTypeKeyword}, // array
				"artifacts": {Type: propTypeKeyword}, // array
				"host":      {Type: propTypeKeyword},
			},
		},
		"deploy": {
			Properties: map[string]mappingProp{
				"install": {
					Properties: map[string]mappingProp{
						"commands": {Type: propTypeKeyword}, // array
					},
				},
				"run": {
					Properties: map[string]mappingProp{
						"commands":    {Type: propTypeKeyword}, // array
						"autoRestart": {Type: propTypeBool},
					},
				},
				"target": {
					Properties: map[string]mappingProp{
						"ids":  {Type: propTypeKeyword}, // array
						"tags": {Type: propTypeKeyword}, // array
					},
				},
				"match": {
					Properties: map[string]mappingProp{
						"ids":  {Type: propTypeKeyword}, // array
						"tags": {Type: propTypeKeyword}, // array
						"list": {Type: propTypeKeyword}, // array
					},
				},
			},
		},
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

func (s *storage) AddTarget(target *Target) error {
	res, err := s.client.Index().Index(indexTarget).Type(typeFixed).
		Id(target.ID).BodyJson(target).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s/%s v%d", res.Index, res.Id, res.Version)
	return nil
}

// PathTarget updates fields that are not omitted, returns false if target is not found
func (s *storage) PatchTarget(id string, target *Target) (bool, error) {
	res, err := s.client.Update().Index(indexTarget).Type(typeFixed).Id(id).Doc(target).Do(s.ctx)
	if err != nil {
		e := err.(*elastic.Error)
		if e.Status == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	log.Printf("Updated %s/%s v%d", res.Index, res.Id, res.Version)
	return true, nil
}

func (s *storage) GetTargets(tags []string, from, size int) ([]Target, int64, error) {

	query := elastic.NewBoolQuery()
	for i := range tags {
		query.Must(elastic.NewMatchQuery("tags", tags[i]))
	}

	searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
		Query(query).Sort("id", true).From(from).Size(size).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var targets []Target
	if searchResult.Hits.TotalHits > 0 {
		//log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var target Target
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

// MatchTargets searches for targets with IDs and tags and returns a list of tags and ids covering all the matches
// The search result gives priority to tags (in the given order) and then IDs
func (s *storage) MatchTargets(ids, tags []string) (allIDs, matchIDs, matchTags []string, err error) {
	if len(ids)+len(tags) == 0 {
		return
	}

	// highlight tags, sorted by the score (boost value)
	highlight := elastic.NewHighlight().
		PreTags("").PostTags("").Order("score").HighlighterType("plain").Field("tags")

	query := elastic.NewBoolQuery()
	// add tags to query (with boost)
	boost := len(tags)
	for i, tag := range tags {
		query.Should(elastic.NewMatchQuery("tags", tag).Boost(float64(boost - i)))
	}
	// add ids to query
	for _, id := range ids {
		query.Should(elastic.NewMatchQuery("id", id))
	}

	hitIDs, hitTags := make(map[string]bool), make(map[string]bool)
	const perPage = 100
	for from := 0; ; from += perPage {

		searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
			Query(query).Highlight(highlight).Sort("id", true).From(from).Size(perPage).FetchSource(false).Do(s.ctx)
		if err != nil {
			return nil, nil, nil, err
		}

		if searchResult.Hits.TotalHits > 0 {
			//log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

			for _, hit := range searchResult.Hits.Hits {
				allIDs = append(allIDs, hit.Id)
				if highlights, ok := hit.Highlight["tags"]; ok {
					if len(highlights) < 1 {
						return nil, nil, nil, fmt.Errorf("unexpected search response. Highlights has length 0")
					}
					hitTags[highlights[0]] = true
				} else {
					hitIDs[hit.Id] = true
				}
			}
		} else {
			log.Print("Found no entries")
		}

		if int64(len(searchResult.Hits.Hits)) >= searchResult.Hits.TotalHits {
			break
		}
	}

	for id := range hitIDs {
		matchIDs = append(matchIDs, id)
	}
	for tag := range hitTags {
		matchTags = append(matchTags, tag)
	}
	return
}

// SearchTargets takes an Elastic Search's Request Body to perform any query on the index
// Request body should follow: https://www.elastic.co/guide/en/elasticsearch/reference/current/search-request-body.html
func (s *storage) SearchTargets(source map[string]interface{}) ([]Target, int64, error) {

	searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
		Source(source).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var targets []Target
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)
		for _, hit := range searchResult.Hits.Hits {
			var t Target
			err := json.Unmarshal(*hit.Source, &t)
			if err != nil {
				return nil, 0, err
			}
			targets = append(targets, t)
		}
	} else {
		log.Print("Found no entries")
	}
	return targets, searchResult.Hits.TotalHits, nil
}

func (s *storage) GetTarget(id string) (*Target, error) {
	res, err := s.client.Get().Index(indexTarget).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		return nil, err
	}
	if res.Found {
		//log.Printf("Got document %s/%s v%d", res.Index, res.Id, res.Version)
		var target Target
		err = json.Unmarshal(*res.Source, &target)
		if err != nil {
			return nil, err
		}
		return &target, nil
	}
	log.Printf("Target not found: %s", id)
	return nil, nil
}

func (s *storage) AddOrder(order *Order) error {
	res, err := s.client.Index().Index(indexOrder).Type(typeFixed).
		Id(order.ID).BodyJson(order).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s/%s v%d", res.Index, res.Id, res.Version)
	return nil
}

func (s *storage) GetOrders(from, size int) ([]Order, int64, error) {
	searchResult, err := s.client.Search().Index(indexOrder).Type(typeFixed).
		Sort("id", true).From(from).Size(size).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var orders []Order
	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		for _, hit := range searchResult.Hits.Hits {
			var order Order
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

func (s *storage) GetOrder(id string) (*Order, error) {
	res, err := s.client.Get().Index(indexOrder).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		return nil, err
	}
	if res.Found {
		//log.Printf("Got document %s/%s v%d", res.Index, res.Id, res.Version)
		var order Order
		err = json.Unmarshal(*res.Source, &order)
		if err != nil {
			return nil, err
		}
		return &order, nil
	}
	log.Printf("Order not found: %s", id)
	return nil, nil
}

func (s *storage) GetLogs(target, task, stage, command, output, error, sortField string, sortAsc bool, from, size int) ([]Log, int64, error) {
	query := elastic.NewBoolQuery()
	if target != "" {
		query.Must(elastic.NewMatchQuery("target", target))
	}
	if task != "" {
		query.Must(elastic.NewMatchQuery("task", task))
	}
	if stage != "" {
		query.Must(elastic.NewMatchQuery("stage", stage))
	}
	if command != "" {
		query.Must(elastic.NewMatchQuery("command", command))
	}
	if error == "true" {
		query.Must(elastic.NewMatchQuery("error", error))
	}
	if output != "" { // output has text type
		query.Must(elastic.NewMatchQuery("output", output).Operator("and"))
	}

	searchResult, err := s.client.Search().Index(indexLog).Type(typeFixed).
		Query(query).Sort(sortField, sortAsc).From(from).Size(size).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var logs []Log
	if searchResult.Hits.TotalHits > 0 {
		//log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)
		for _, hit := range searchResult.Hits.Hits {
			var l Log
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

// SearchLogs takes an Elastic Search's Request Body to perform any query on the index
// Request body should follow: https://www.elastic.co/guide/en/elasticsearch/reference/current/search-request-body.html
func (s *storage) SearchLogs(source map[string]interface{}) ([]Log, int64, error) {

	searchResult, err := s.client.Search().Index(indexLog).Type(typeFixed).
		Source(source).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	var logs []Log
	if searchResult.Hits.TotalHits > 0 {
		//log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)
		for _, hit := range searchResult.Hits.Hits {
			var l Log
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

func (s *storage) AddLog(logM *Log) error {
	res, err := s.client.Index().Index(indexLog).Type(typeFixed).
		BodyJson(logM).Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %s from %s/%s", res.Index, logM.Target, logM.Task)
	return nil
}

func (s *storage) AddLogs(logs []Log) error {
	bulk := s.client.Bulk()
	for i := range logs {
		bulk.Add(elastic.NewBulkIndexRequest().Index(indexLog).Type(typeFixed).Doc(logs[i]))
	}
	res, err := bulk.Do(s.ctx)
	if err != nil {
		return err
	}
	log.Printf("Indexed %d log(s)", len(res.Indexed()))
	return nil
}

// DeliveredTask returns true if the task is received by the target
//	i.e. there is any log from the target for the task
func (s *storage) DeliveredTask(target, task string) (bool, error) {

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchQuery("target", target)).
		Must(elastic.NewMatchQuery("task", task)).
		Must(elastic.NewMatchQuery("command", model.CommandByAgent))

	searchResult, err := s.client.Search().Index(indexLog).Type(typeFixed).
		Query(query).From(0).Size(1).FetchSource(false).Do(s.ctx) // TODO change size to 0?
	if err != nil {
		return false, err
	}

	return searchResult.Hits.TotalHits > 0, nil
}

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"github.com/olivere/elastic"
)

type Storage interface {
	GetOrders(descr string, sortAsc bool, from, size int) ([]Order, int64, error)
	AddOrder(*Order) error
	GetOrder(id string) (*Order, error)
	DeleteOrder(id string) (found bool, err error)
	//
	GetTargets(tags []string, from, size int) ([]Target, int64, error)
	GetTargetKeys() (map[string]string, error)
	PatchTarget(id string, target *Target) (found bool, err error)
	IndexTarget(target *Target) (found bool, err error) // add or update
	MatchTargets(ids, tags []string) (allIDs, hitIDs, hitTags []string, err error)
	AddTargetTrans(*Target) (conflict bool, trans *transaction, err error)
	GetTarget(id string) (*Target, error)
	DeleteTarget(id string) (*Target, error)
	//
	GetLogs(target, task, stage, command, output, error, sortField string, sortAsc bool, from, size int) ([]Log, int64, error)
	AddLog(*Log) error
	AddLogs(logs []Log) error
	DeleteLogs(target, task string) error
	DeliveredTask(target, task string) (delivered bool, err error)
	//
	GetTokens(name string) ([]TokenMeta, error)
	AddToken(TokenHashed) (duplicate bool, err error)
	findToken(hash string) (found bool, err error)
	DeleteTokenTrans(hash string) (found bool, trans *transaction, err error)
	DeleteTokens(name string) error
	//
	DoBulk(...interface{}) error
}

type transaction struct {
	Commit  interface{}
	Release func()
}

type storage struct {
	client *elastic.Client
	ctx    context.Context
	// mutex locks
	targetLocker sync.Mutex
	tokenLocker  sync.Mutex
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
	envElasticDebug  = "DEBUG_ELASTIC"
	indexTarget      = "target"
	indexOrder       = "order"
	indexLog         = "log"
	indexToken       = "token"
	typeFixed        = "_doc"
	mappingStrict    = "strict"
	propTypeKeyword  = "keyword"
	propTypeDate     = "date"
	propTypeText     = "text"
	propTypeBool     = "boolean"
	propTypeGeoPoint = "geo_point"
)

// StartElasticStorage starts an elastic storage client. It
//  - creates an elastic client
//  - waits for the server (few attempts)
//  - creates storage indices (if missing)
func StartElasticStorage(url string) (Storage, error) {
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

	// Wait for Elasticsearch server
	const maxAttempts = 3
	for attempts := 1; ; attempts++ {
		info, code, err := client.Ping(url).Do(ctx)
		if err != nil {
			log.Printf("Elasticsearch ping error (attempt %d/%d): %s", attempts, maxAttempts, err)
			if attempts < maxAttempts {
				time.Sleep(time.Duration(attempts*10) * time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to reach Elasticsearch within %d attempts", maxAttempts)
		}
		log.Printf("Elasticsearch returned with code %d and version %s", code, info.Version.Number)
		break
	}

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
		"location":     {Type: propTypeGeoPoint},
		"publicKey":    {Type: propTypeKeyword},
		"createdAt":    {Type: propTypeDate},
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
		"id":          {Type: propTypeKeyword},
		"debug":       {Type: propTypeBool},
		"description": {Type: propTypeText},
		"createdAt":   {Type: propTypeDate},
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

	m = mapping{}
	m.Settings.Shards = 1
	m.Settings.Replicas = 0
	m.Settings.RefreshInterval = "1s"
	m.Mappings.Doc.Dynamic = mappingStrict
	m.Mappings.Doc.Prop = map[string]mappingProp{
		"name":      {Type: propTypeKeyword},
		"hash":      {Type: propTypeKeyword},
		"expiresAt": {Type: propTypeDate},
	}
	err = s.createIndex(indexToken, m)
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

// AddTargetTrans prepares a create operation and returns an object to commit and/or release the transaction
func (s *storage) AddTargetTrans(target *Target) (conflict bool, trans *transaction, err error) {
	if target, err := s.GetTarget(target.ID); err != nil {
		return false, nil, err
	} else if target != nil {
		return true, nil, nil
	}

	s.targetLocker.Lock()
	trans = &transaction{
		Commit: elastic.NewBulkIndexRequest().Index(indexTarget).Type(typeFixed).
			Id(target.ID).OpType("create").Doc(target),
		Release: func() {
			s.targetLocker.Unlock()
		},
	}

	return false, trans, nil
}

// PatchTarget updates fields that are not omitted, returns false if target is not found
func (s *storage) PatchTarget(id string, target *Target) (found bool, err error) {
	res, err := s.client.Update().Index(indexTarget).Type(typeFixed).Id(id).Doc(target).Do(s.ctx)
	if err != nil {
		e := err.(*elastic.Error)
		if e.Status == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	log.Printf("Patched %s/%s v%d", res.Index, res.Id, res.Version)
	return true, nil
}

// IndexTarget adds or updates the target
func (s *storage) IndexTarget(target *Target) (found bool, err error) {
	res, err := s.client.Index().Index(indexTarget).Type(typeFixed).
		Id(target.ID).BodyJson(target).Do(s.ctx)
	if err != nil {
		return false, err
	}
	log.Printf("Indexed %s/%s v%d", res.Index, res.Id, res.Version)
	return true, nil
}

func (s *storage) GetTargets(tags []string, from, size int) (targets []Target, total int64, err error) {

	query := elastic.NewBoolQuery()
	for i := range tags {
		query.Must(elastic.NewMatchQuery("tags", tags[i]))
	}

	searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
		Query(query).Sort("id", true).From(from).Size(size).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	if searchResult.Hits.TotalHits > 0 {
		//log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		targets = make([]Target, searchResult.Hits.TotalHits)
		for i, hit := range searchResult.Hits.Hits {
			err := json.Unmarshal(*hit.Source, &targets[i])
			if err != nil {
				return nil, 0, err
			}
		}
	} else {
		log.Print("Found no entries")
	}
	return targets, searchResult.Hits.TotalHits, nil
}

func (s *storage) GetTargetKeys() (map[string]string, error) {
	fetch := elastic.NewFetchSourceContext(true).Include("publicKey")

	keys := make(map[string]string)
	size := 100
	fetched := 0
	for from := 0; ; from += size {
		searchResult, err := s.client.Search().Index(indexTarget).Type(typeFixed).
			FetchSourceContext(fetch).From(from).Size(size).Do(s.ctx)
		if err != nil {
			return nil, err
		}

		if searchResult.Hits.TotalHits > 0 {
			for _, hit := range searchResult.Hits.Hits {
				var target Target
				err := json.Unmarshal(*hit.Source, &target)
				if err != nil {
					return nil, err
				}
				if target.PublicKey != "" {
					keys[hit.Id] = target.PublicKey
				} else {
					log.Println("Warning: no public key for target:", hit.Id)
				}
			}
			fetched += len(searchResult.Hits.Hits)
		}
		if searchResult.Hits.TotalHits <= int64(fetched) {
			break
		}
	}
	return keys, nil
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

func (s *storage) GetTarget(id string) (*Target, error) {
	res, err := s.client.Get().Index(indexTarget).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		e := err.(*elastic.Error)
		if e.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	//log.Printf("Got document %s/%s v%d", res.Index, res.Id, res.Version)
	var target Target
	err = json.Unmarshal(*res.Source, &target)
	if err != nil {
		return nil, err
	}
	return &target, nil
}

func (s *storage) DeleteTarget(id string) (*Target, error) {
	target, err := s.GetTarget(id)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, nil
	}

	res, err := s.client.Delete().Index(indexTarget).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("Deleted %s/%s v%d", res.Index, res.Id, res.Version)
	return target, nil
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

func (s *storage) GetOrders(descr string, sortAsc bool, from, size int) (orders []Order, total int64, err error) {
	query := elastic.NewBoolQuery()
	if descr != "" { // description has text type
		query.Must(elastic.NewMatchQuery("description", descr).Operator("and"))
	}

	searchResult, err := s.client.Search().Index(indexOrder).Type(typeFixed).
		Query(query).Sort("createdAt", sortAsc).From(from).Size(size).Do(s.ctx)
	if err != nil {
		return nil, 0, err
	}

	if searchResult.Hits.TotalHits > 0 {
		log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		orders = make([]Order, searchResult.Hits.TotalHits)
		for i, hit := range searchResult.Hits.Hits {
			err := json.Unmarshal(*hit.Source, &orders[i])
			if err != nil {
				return nil, 0, err
			}
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
		e := err.(*elastic.Error)
		if e.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	//log.Printf("Got document %s/%s v%d", res.Index, res.Id, res.Version)
	var order Order
	err = json.Unmarshal(*res.Source, &order)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *storage) DeleteOrder(id string) (found bool, err error) {
	res, err := s.client.Delete().Index(indexOrder).Type(typeFixed).
		Id(id).Do(s.ctx)
	if err != nil {
		e := err.(*elastic.Error)
		if e.Status == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	log.Printf("Deleted %s/%s v%d", res.Index, res.Id, res.Version)
	return true, nil
}

func (s *storage) GetLogs(target, task, stage, command, output, error, sortField string, sortAsc bool, from, size int) (logs []Log, total int64, err error) {
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

	if searchResult.Hits.TotalHits > 0 {
		//log.Printf("Found %d entries in %dms", searchResult.Hits.TotalHits, searchResult.TookInMillis)

		logs = make([]Log, searchResult.Hits.TotalHits)
		for i, hit := range searchResult.Hits.Hits {
			err := json.Unmarshal(*hit.Source, &logs[i])
			if err != nil {
				return nil, 0, err
			}
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

func (s *storage) DeleteLogs(target, task string) error {
	query := elastic.NewBoolQuery()
	if target != "" {
		query.Must(elastic.NewMatchQuery("target", target))
	}
	if task != "" {
		query.Must(elastic.NewMatchQuery("task", task))
	}

	_, err := s.client.DeleteByQuery(indexLog).Query(query).Do(s.ctx)
	if err != nil {
		return err
	}
	return nil
}

// DeliveredTask returns true if the task is received by the target
//	i.e. there is any log from the target for the task
func (s *storage) DeliveredTask(target, task string) (delivered bool, err error) {

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

func (s *storage) GetTokens(name string) ([]TokenMeta, error) {
	query := elastic.NewBoolQuery()
	if name != "" {
		query.Must(elastic.NewMatchQuery("name", name))
	}

	// TODO paginate or use the scroll service
	searchResult, err := s.client.Search().Index(indexToken).Type(typeFixed).
		Query(query).Size(1000).Sort("expiresAt", false).Do(s.ctx)
	if err != nil {
		return nil, err
	}

	var tokens []TokenMeta
	if searchResult.Hits.TotalHits > 0 {
		tokens = make([]TokenMeta, searchResult.Hits.TotalHits)
		for i, hit := range searchResult.Hits.Hits {
			err := json.Unmarshal(*hit.Source, &tokens[i])
			if err != nil {
				return nil, err
			}
		}
	} else {
		log.Print("Found no entries")
	}
	return tokens, nil
}

func (s *storage) AddToken(token TokenHashed) (duplicate bool, err error) {
	res, err := s.client.Index().Index(indexToken).Type(typeFixed).
		Id(string(token.Hash)).OpType("create").BodyJson(token).Do(s.ctx)
	if err != nil {
		e := err.(*elastic.Error)
		if e.Status == http.StatusConflict {
			return true, nil
		}
		return false, err
	}
	log.Printf("Indexed %s/%s v%d", res.Index, res.Id, res.Version)
	return false, nil
}

func (s *storage) findToken(hash string) (found bool, err error) {
	_, err = s.client.Get().Index(indexToken).Type(typeFixed).FetchSource(false).
		Id(hash).Do(s.ctx)
	if err != nil {
		e := err.(*elastic.Error)
		if e.Status == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
	//log.Printf("Got %s/%s v%d", res.Index, res.Id, res.Version)
}

// DeleteTokenTrans prepares a delete operation and returns an object to commit and/or release the transaction
func (s *storage) DeleteTokenTrans(hash string) (found bool, trans *transaction, err error) {
	found, err = s.findToken(hash)
	if err != nil || !found {
		return false, nil, err
	}

	s.tokenLocker.Lock()
	trans = &transaction{
		Commit: elastic.NewBulkDeleteRequest().Index(indexToken).Type(typeFixed).Id(hash),
		Release: func() {
			s.tokenLocker.Unlock()
		},
	}

	return true, trans, nil
}

func (s *storage) DeleteTokens(name string) error {
	query := elastic.NewBoolQuery().Must(elastic.NewMatchQuery("name", name))

	_, err := s.client.DeleteByQuery(indexToken).Query(query).Do(s.ctx)
	if err != nil {
		return err
	}
	return nil
}

// DoBulk performs elastic.BulkableRequests
func (s *storage) DoBulk(requests ...interface{}) error {
	bulk := s.client.Bulk()
	for i := range requests {
		bulk.Add(requests[i].(elastic.BulkableRequest))
	}
	res, err := bulk.Do(s.ctx)
	if err != nil {
		return err
	}
	var errors []string
	if res.Errors {
		for _, item := range res.Items {
			for operation, result := range item {
				if result.Error != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", operation, result.Error))
				}
			}
		}
		return fmt.Errorf(strings.Join(errors, ","))
	}
	return nil
}

package redis

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/henomis/lingoose/index"
	"github.com/henomis/lingoose/index/option"

	"github.com/RediSearch/redisearch-go/v2/redisearch"
)

const (
	errUnknownIndexName = "Unknown index name"
)

type DB struct {
	redisearchClient *redisearch.Client
	includeContent   bool
	includeValues    bool

	createIndex *CreateIndexOptions
}

type Distance string

const (
	DistanceCosine    Distance = "COSINE"
	DistanceEuclidean Distance = "IP"
	DistanceDot       Distance = "L2"

	defaultVectorFieldName      = "vec"
	defaultVectorScoreFieldName = "__vec_score"
)

type CreateIndexOptions struct {
	Dimension uint64
	Distance  Distance
}

type Options struct {
	RedisearchClient *redisearch.Client
	IncludeContent   bool
	IncludeValues    bool

	CreateIndex *CreateIndexOptions
}

func New(options Options) *DB {
	return &DB{
		redisearchClient: options.RedisearchClient,
		includeContent:   options.IncludeContent,
		includeValues:    options.IncludeValues,
		createIndex:      options.CreateIndex,
	}
}

func (d *DB) IsEmpty(ctx context.Context) (bool, error) {
	err := d.createIndexIfRequired(ctx)
	if err != nil {
		return true, fmt.Errorf("%w: %w", index.ErrInternal, err)
	}

	indexInfo, err := d.redisearchClient.Info()
	if err != nil {
		return true, fmt.Errorf("%w: %w", index.ErrInternal, err)
	}

	return indexInfo.DocCount == 0, nil
}

func (d *DB) Insert(ctx context.Context, datas []index.Data) error {
	err := d.createIndexIfRequired(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", index.ErrInternal, err)
	}

	var documents []redisearch.Document
	for _, data := range datas {
		if data.ID == "" {
			id, errUUID := uuid.NewUUID()
			if errUUID != nil {
				return errUUID
			}
			data.ID = id.String()
		}

		document := redisearch.NewDocument(data.ID, 1.0)

		for key, value := range data.Metadata {
			document.Set(key, value)
		}

		document.Set(defaultVectorFieldName, float64tobytes(data.Values))

		documents = append(documents, document)
	}

	if err := d.redisearchClient.Index(documents...); err != nil {
		return fmt.Errorf("%w: %w", index.ErrInternal, err)
	}

	return nil
}

func (d *DB) Search(ctx context.Context, values []float64, options *option.Options) (index.SearchResults, error) {
	matches, err := d.similaritySearch(ctx, values, options)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", index.ErrInternal, err)
	}

	return buildSearchResultsFromQdrantMatches(matches, d.includeContent), nil
}

func (d *DB) similaritySearch(
	ctx context.Context,
	values []float64,
	opts *option.Options,
) ([]redisearch.Document, error) {
	if opts.Filter == nil {
		opts.Filter = redisearch.Filter{}
	}

	docs, _, err := d.redisearchClient.Search(
		redisearch.NewQuery(fmt.Sprintf("(*)=>[KNN %d @vec $query_vector]", opts.TopK)).
			SetSortBy(defaultVectorScoreFieldName, true).
			SetFlags(redisearch.QueryWithPayloads).
			SetDialect(2).
			Limit(0, opts.TopK).
			AddParam("query_vector", float64tobytes(values)).
			AddFilter(opts.Filter.(redisearch.Filter)),
	)

	return docs, err
}

func (d *DB) createIndexIfRequired(ctx context.Context) error {
	if d.createIndex == nil {
		return nil
	}

	indexName := ""
	indexInfo, err := d.redisearchClient.Info()
	if err != nil && (err.Error() != errUnknownIndexName) {
		return err
	} else if err == nil {
		indexName = indexInfo.Name
	}

	indexes, err := d.redisearchClient.List()
	if err != nil {
		return err
	}

	if len(indexes) > 0 && len(indexName) > 0 {
		for _, index := range indexes {
			if index == indexInfo.Name {
				return nil
			}
		}
	}

	return d.redisearchClient.CreateIndex(
		redisearch.NewSchema(redisearch.DefaultOptions).
			AddField(redisearch.NewVectorFieldOptions(
				defaultVectorFieldName,
				redisearch.VectorFieldOptions{
					Algorithm: redisearch.Flat,
					Attributes: map[string]interface{}{
						"TYPE":            "FLOAT32",
						"DIM":             d.createIndex.Dimension,
						"DISTANCE_METRIC": d.createIndex.Distance,
					}})),
	)
}

func buildSearchResultsFromQdrantMatches(
	matches []redisearch.Document,
	includeContent bool,
) index.SearchResults {
	searchResults := make([]index.SearchResult, len(matches))

	for i, match := range matches {
		metadata := index.DeepCopyMetadata(match.Properties)
		if !includeContent {
			delete(metadata, index.DefaultKeyContent)
		}

		score := 0.0
		scoreField, ok := match.Properties[defaultVectorScoreFieldName]
		if ok {
			scoreAsString, ok := scoreField.(string)
			if ok {
				score, _ = strconv.ParseFloat(scoreAsString, 64)
				delete(metadata, defaultVectorScoreFieldName)
			}
		}

		values := []float64{}
		vectorField, ok := match.Properties[defaultVectorFieldName]
		if ok {
			vectorAsString, ok := vectorField.(string)
			if ok {
				values = bytestofloat64([]byte(vectorAsString))
				delete(metadata, defaultVectorFieldName)
			}
		}

		searchResults[i] = index.SearchResult{
			Data: index.Data{
				ID:       match.Id,
				Metadata: metadata,
				Values:   values,
			},
			Score: score,
		}
	}

	return searchResults
}

func float64to32(floats []float64) []float32 {
	floats32 := make([]float32, len(floats))
	for i, f := range floats {
		floats32[i] = float32(f)
	}
	return floats32
}

func float64tobytes(floats64 []float64) []byte {
	floats := float64to32(floats64)

	byteSlice := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(byteSlice[i*4:], bits)
	}
	return byteSlice
}
func bytestofloat64(byteSlice []byte) []float64 {
	floats := make([]float32, len(byteSlice)/4)
	for i := 0; i < len(byteSlice); i += 4 {
		bits := binary.LittleEndian.Uint32(byteSlice[i : i+4])
		floats[i/4] = math.Float32frombits(bits)
	}

	floats64 := make([]float64, len(floats))
	for i, f := range floats {
		floats64[i] = float64(f)
	}
	return floats64
}

package clients

import (
	"ai-assistant/internal/models"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

const ClusterEndpoint = "a78c2e5b-7d6f-4ee7-9d85-d1b2a39b981e.europe-west3-0.gcp.cloud.qdrant.io"
const Port = 6334
const VectorSize = 768
const CollectionName = "devi-public-info"
const DistanceMetric = qdrant.Distance_Cosine
const MaxRetries = 5
const InitialRetryDelay = 1 * time.Second

type QdrantClient struct {
	apiKey string
	client *qdrant.Client
}

func NewQdrantClient(apiKey string) *QdrantClient {
	config := qdrant.Config{
		Host:   ClusterEndpoint,
		Port:   Port,
		APIKey: apiKey,
		UseTLS: true,
	}

	client, err := qdrant.NewClient(&config)
	if err != nil {
		panic("Failed to create Qdrant client: " + err.Error())
	}

	return &QdrantClient{
		apiKey: apiKey,
		client: client,
	}
}

func (qc *QdrantClient) GetClient() *qdrant.Client {
	return qc.client
}

// retryWithBackoff executes a function with exponential backoff retry logic
func (qc *QdrantClient) retryWithBackoff(operation func() error, operationName string) error {
	var lastErr error
	delay := InitialRetryDelay

	for attempt := 0; attempt < MaxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("Attempt %d/%d failed for %s: %v", attempt+1, MaxRetries, operationName, err)

		if attempt < MaxRetries-1 {
			log.Printf("Retrying in %v...", delay)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operationName, MaxRetries, lastErr)
}

func (qc *QdrantClient) UpsertSinglePoint(pointId uint64, vector []float32, payload models.Payload) error {
	var result *qdrant.UpdateResult

	err := qc.retryWithBackoff(func() error {
		res, err := qc.client.Upsert(context.Background(), &qdrant.UpsertPoints{
			CollectionName: CollectionName,
			Points: []*qdrant.PointStruct{
				{
					Id:      qdrant.NewIDNum(pointId),
					Vectors: qdrant.NewVectors(vector...),
					Payload: payload.ToMap(),
				},
			},
		})

		if err != nil {
			return err
		}

		result = res
		return nil
	}, "UpsertSinglePoint")

	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func (qc *QdrantClient) SearchSimilarVectors(vector []float32, topK int) ([]models.Payload, error) {
	limit := uint64(topK)
	var payloads []models.Payload

	err := qc.retryWithBackoff(func() error {
		res, err := qc.client.Query(context.Background(), &qdrant.QueryPoints{
			CollectionName: CollectionName,
			Query:          qdrant.NewQuery(vector...),
			Limit:          &limit,
			WithPayload:    qdrant.NewWithPayload(true),
		})

		if err != nil {
			return err
		}

		// Clear previous results
		payloads = nil
		for _, scoredPoint := range res {
			payloadMap := scoredPoint.Payload
			payloads = append(payloads, models.FromMap(payloadMap))
		}

		return nil
	}, "SearchSimilarVectors")

	if err != nil {
		return nil, err
	}

	return payloads, nil
}

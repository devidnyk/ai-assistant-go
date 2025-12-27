package operations

import (
	"ai-assistant/internal/app"
	"ai-assistant/internal/common"
	"context"
	"fmt"
	"log"
)

type QueryOperation struct {
	Query      string
	AppContext *app.AppContext
}

func InitOperation() *QueryOperation {
	return &QueryOperation{}
}

func (qo *QueryOperation) RunOperation() {
	query := qo.Query
	ctx := qo.AppContext

	qembeddings, err := ctx.GeminiClient.GetEmbeddingSingle(query, context.TODO())
	if err != nil {
		panic(err)
	}

	res, err := ctx.QdrantClient.SearchSimilarVectors(qembeddings, 4)
	if err != nil {
		panic(err)
	}

	queryContext := make([]string, 0)

	for _, payload := range res {
		log.Println("Context used: ", payload.DataSource, payload.SourceType, payload.Context)
		text, _, err := common.ReadFile(payload.SourceType, payload.DataSource)
		if err != nil {
			log.Panic("Failed to read file: ", payload.DataSource, err)
		}

		queryContext = append(queryContext, fmt.Sprintf("Source file path: %s | What is this: %s \n\nFile Content: %s", payload.DataSource, payload.Context, text))
	}

	qres, err := ctx.GeminiClient.GetResponseWithContext(query, queryContext, context.Background())

	log.Printf("Query: %s\nResponse: %s\n", query, qres)
}

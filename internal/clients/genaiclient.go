package clients

import (
	"context"
	"log"
	"os"

	"google.golang.org/genai"
)

const EmbeddingModelId = "gemini-embedding-001"
const EmbeddingDimension int32 = 768

type GenaiClient struct {
	apiKey    string
	client    *genai.Client
	sysConfig *genai.GenerateContentConfig
}

func NewGenAiClientWithApiKey(apiKey string, sysPromptPath string) *GenaiClient {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		panic("Failed to create GenAI client: " + err.Error())
	}

	promptBytes, err := os.ReadFile(sysPromptPath)
	if err != nil {
		panic("Failed to read system prompt file: " + err.Error())
	}

	temp := float32(0.5)
	sysCfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(
			string(promptBytes),
			genai.RoleModel,
		),
		MaxOutputTokens: 2048,
		Temperature:     &temp,
	}

	return &GenaiClient{
		apiKey:    apiKey,
		client:    client,
		sysConfig: sysCfg,
	}
}

func (gc *GenaiClient) GetClient() *genai.Client {
	return gc.client
}

func (gc *GenaiClient) ResetSysPrompt(sysPrompt string) {
	gc.sysConfig.SystemInstruction = genai.NewContentFromText(sysPrompt, genai.RoleModel)
}

func (gc *GenaiClient) GetEmbeddingSingle(input string, ctx context.Context) ([]float32, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(input, genai.RoleUser),
	}

	embeddingDim := EmbeddingDimension
	result, err := gc.client.Models.EmbedContent(ctx,
		EmbeddingModelId,
		contents,
		&genai.EmbedContentConfig{OutputDimensionality: &embeddingDim},
	)
	if err != nil {
		log.Fatal(err)
	}

	return result.Embeddings[0].Values, nil
}

func (gc *GenaiClient) GetResponseSingle(query string, ctx context.Context) (string, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(query, genai.RoleUser),
	}

	generateCfg := &genai.GenerateContentConfig{
		SystemInstruction: gc.sysConfig.SystemInstruction,
		MaxOutputTokens:   256,
	}

	result, err := gc.client.Models.GenerateContent(ctx,
		"gemini-2.5-flash",
		contents,
		generateCfg,
	)

	if err != nil {
		log.Fatal(err)
	}

	return result.Text(), nil
}

// GetResponseWithContext uses retrieved context from vector DB to answer the query
func (gc *GenaiClient) GetResponseWithContext(query string, contextData []string, ctx context.Context) (string, error) {
	// Build the prompt with context
	var prompt string
	if len(contextData) > 0 {
		prompt = "Context information from the knowledge base:\n\n"
		for i, context := range contextData {
			prompt += "--- Context " + string(rune(i+1)) + " ---\n"
			prompt += context + "\n\n"
		}
		prompt += "Based on the above context, please answer the following question:\n"
		prompt += query
	} else {
		prompt = query
	}

	contents := []*genai.Content{
		genai.NewContentFromText(prompt, genai.RoleUser),
	}

	result, err := gc.client.Models.GenerateContent(ctx,
		"gemini-2.5-flash",
		contents,
		gc.sysConfig,
	)

	if err != nil {
		return "", err
	}

	return result.Text(), nil
}

// // GetResponseWithDetailedContext uses retrieved context with metadata for better responses
// func (gc *GenaiClient) GetResponseWithDetailedContext(query string, contextChunks []ContextChunk, ctx context.Context) (string, error) {
// 	// Build enriched prompt with source information
// 	var prompt string
// 	if len(contextChunks) > 0 {
// 		prompt = "Relevant information from your documents:\n\n"
// 		for i, chunk := range contextChunks {
// 			prompt += "--- Source " + string(rune(i+1)) + ": " + chunk.Source + " ---\n"
// 			prompt += chunk.Text + "\n"
// 			if chunk.Metadata != "" {
// 				prompt += "(Metadata: " + chunk.Metadata + ")\n"
// 			}
// 			prompt += "\n"
// 		}
// 		prompt += "Question: " + query + "\n\n"
// 		prompt += "Please provide a detailed answer based on the sources above. Include source references in your answer."
// 	} else {
// 		prompt = query
// 	}

// 	contents := []*genai.Content{
// 		genai.NewContentFromText(prompt, genai.RoleUser),
// 	}

// 	generateCfg := &genai.GenerateContentConfig{
// 		SystemInstruction: genai.NewContentFromText(
// 			"You are a knowledgeable personal assistant with access to the user's documents. "+
// 				"Answer questions accurately using ONLY the provided context. "+
// 				"Always cite your sources (e.g., 'According to Source 1...'). "+
// 				"If the context is insufficient, acknowledge what you don't know. "+
// 				"Be concise but thorough.",
// 			genai.RoleModel,
// 		),
// 		MaxOutputTokens: 1024,
// 		Temperature:     0.2, // Very low for factual accuracy
// 		TopP:            0.8,
// 		TopK:            40,
// 	}

// 	result, err := gc.client.Models.GenerateContent(ctx,
// 		"gemini-2.5-flash",
// 		contents,
// 		generateCfg,
// 	)

// 	if err != nil {
// 		return "", err
// 	}

// 	return result.Text(), nil
// }

// // ContextChunk represents a piece of context retrieved from vector DB
// type ContextChunk struct {
// 	Text     string
// 	Source   string
// 	Metadata string
// 	Score    float32
// }

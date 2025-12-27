package setup

import (
	"ai-assistant/configs"
	"ai-assistant/internal/clients"
	"ai-assistant/internal/common"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func processFile(filePath string, sourcetype configs.SourceDataType, geminiClient *clients.GenaiClient, qdrantClient *clients.QdrantClient) error {
	log.Println("Processing file: ", filePath, sourcetype)
	content, payload, err := common.ReadFile(sourcetype, filePath)

	if err != nil {
		log.Panic("Failed to Process file: ", filePath, sourcetype)
	}

	emebddings, err := geminiClient.GetEmbeddingSingle(content, context.Background())
	if err != nil {
		log.Panic("Failed to get embedding for file: ", filePath, err)
	}

	fileNameId := fmt.Sprintf("%s-%s", sourcetype.String(), filePath)
	err = qdrantClient.UpsertSinglePoint(configs.GetHashId(fileNameId), emebddings, *payload)
	if err != nil {
		log.Panic("Failed to upsert point to Qdrant for file: ", filePath, err)
	}

	return nil
}

// GetFilesInFolder returns a list of file paths in a folder
func GetFilesInFolder(folderPath string) ([]string, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(folderPath, entry.Name()))
		}
	}

	return files, nil
}

func SetupUserData(geminiClient *clients.GenaiClient, qdrantClient *clients.QdrantClient, sourceType configs.SourceDataType, sourceDataUrl string) {

	sourceType = configs.Local // For now, only local files are supported

	files, err := GetFilesInFolder(sourceDataUrl)
	if err != nil {
		log.Panic("Failed to get files in folder: ", sourceDataUrl, err)
	}

	for _, filePath := range files {
		err := processFile(filePath, sourceType, geminiClient, qdrantClient)
		if err != nil {
			log.Panic("Failed to process file: ", filePath, err)
		}
	}
}

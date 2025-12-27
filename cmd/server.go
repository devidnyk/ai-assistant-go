package main

import (
	"ai-assistant/configs"
	"ai-assistant/internal/app"
	"ai-assistant/internal/clients"
	"ai-assistant/internal/operations"
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

const sourceDataFolder string = "./myinfo/"

func main() {
	fmt.Println("Starting the application...")
	config := configs.InitConfig()

	log.Println("Initialising clients")
	genaiClient := clients.NewGenAiClientWithApiKey(config.GeminiApiKey)
	telebotClient := clients.NewTeleClient(config.BotToken)
	qdrantClient := clients.NewQdrantClient(config.QdrantApiKey)

	// setup.SetupUserData(genaiClient, qdrantClient, configs.Local, sourceDataFolder)

	appCtx := app.NewAppContext()
	appCtx.InitClients(genaiClient, qdrantClient, telebotClient)

	queryoperation := operations.InitOperation()
	queryoperation.AppContext = appCtx

	// Create reader for stdin
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Enter your query: ")
		userQuery, err := reader.ReadString('\n')
		// userQuery := "how can i get into microsoft like you?"
		if err != nil {
			log.Println("Error reading input:", err)
			continue
		}

		// Trim whitespace and newline
		userQuery = strings.TrimSpace(userQuery)

		if userQuery == "" {
			continue
		}

		queryoperation.Query = userQuery
		queryoperation.RunOperation()
	}
}

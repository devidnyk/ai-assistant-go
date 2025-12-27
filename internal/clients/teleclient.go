package clients

import (
	"context"
	"log"
	"net/http"
)

const BotAPIBaseURL = "https://api.telegram.org/bot"

type TeleClient struct {
	botToken   string
	httpClient *http.Client
}

func NewTeleClient(botToken string) *TeleClient {
	httpClient := http.Client{}
	return &TeleClient{botToken: botToken, httpClient: &httpClient}
}

func NewTeleClientWithHttpClient(botToken string, httpClient *http.Client) *TeleClient {
	return &TeleClient{botToken: botToken, httpClient: httpClient}
}

func (tc *TeleClient) GetBotInfo() (string, error) {
	endpoint := BotAPIBaseURL + tc.botToken + "/getMe"
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		endpoint,
		nil,
	)

	if err != nil {
		log.Panic("Request creation failed. Error: ", err)
	}

	return tc.sendRequest(req)
}

func (tc *TeleClient) GetUpdates() (string, error) {
	endpoint := BotAPIBaseURL + tc.botToken + "/getUpdates"
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		endpoint,
		nil,
	)

	if err != nil {
		log.Panic("Request creation failed. Error: ", err)
	}

	return tc.sendRequest(req)
}

func (tc *TeleClient) sendRequest(request *http.Request) (string, error) {
	log.Default().Println("Sending request to Telegram API:", request.URL.String(), request.Method)
	return SendHttpRequestResponseBase(tc.httpClient, request)
}

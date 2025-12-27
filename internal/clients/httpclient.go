package clients

import (
	"io"
	"net/http"
)

func SendHttpRequestResponseBase(httpClient *http.Client, request *http.Request) (string, error) {
	res, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func SendHttpRequestBase(httpClient *http.Client, request *http.Request) (*http.Response, error) {
	return httpClient.Do(request)
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"myorg/lib/otel"
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	serviceABasePath = "http://localhost:8080"
)

func main() {
	_, _, shutdown := otel.InitTracerMeter(context.Background(), "myclient")
	defer shutdown()

	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	requestServiceA(client, "hello from myclient")
}

func requestServiceA(client http.Client, text string) {
	jsonData := map[string]string{
		"text": text,
	}
	jsonBody, err := json.Marshal(jsonData)
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	url := fmt.Sprintf("%s/echo", serviceABasePath)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	fmt.Println("Response Status:", res.Status)
	fmt.Println("Response Body:", string(body))
}

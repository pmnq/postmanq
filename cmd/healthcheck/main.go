package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%s/health", os.Getenv("PORT")), nil)
	if err != nil {
		cancel()
		os.Exit(1)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		os.Exit(1)
	}

	_ = res.Body.Close()
	cancel()
}

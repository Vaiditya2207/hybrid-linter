//go:build ignore

package main

import (
	"io"
	"net/http"
)

func fetchAPI() string {
	resp, err := http.Get("https://api.example.com/data")
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	return string(body)
}

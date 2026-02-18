package testdata

import (
	"net/http"
)

func FetchData() {
	resp, err := http.Get("http://example.com")
	
	defer resp.Body.Close()
}

package client

import "net/http"

// copyHeader copies the values for the given key from the source header
// to the target request header.
func copyHeader(req *http.Request, src http.Header, key string) {
	if values := src.Values(key); len(values) > 0 {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
}

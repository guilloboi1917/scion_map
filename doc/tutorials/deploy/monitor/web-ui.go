package main

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed index.html
var content embed.FS

func main() {
	// Get the embedded file system
	htmlFS, err := fs.Sub(content, ".")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.FileServer(http.FS(htmlFS)).ServeHTTP(w, r)
	})

	// Generic proxy endpoint
	http.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		target := r.URL.Query().Get("target")
		endpoint := r.URL.Query().Get("endpoint")
		method := r.URL.Query().Get("method")
		if method == "" {
			method = "GET" // Default to GET
		}

		// Validate parameters
		if target == "" || endpoint == "" {
			http.Error(w, "Target and endpoint parameters are required", http.StatusBadRequest)
			return
		}

		// Ensure endpoint starts with a slash
		if !strings.HasPrefix(endpoint, "/") {
			endpoint = "/" + endpoint
		}

		// Build the target URL
		targetURL := "http://" + target + endpoint

		log.Println("Request received - TargetURL: " + targetURL)

		// Create a new request to the target
		var resp *http.Response
		var err error

		switch method {
		case "GET":
			resp, err = http.Get(targetURL)
		case "POST":
			// For POST requests, forward the body and content-type
			contentType := r.Header.Get("Content-Type")
			var body io.Reader
			if r.Body != nil {
				body = r.Body
				defer r.Body.Close()
			}

			resp, err = http.Post(targetURL, contentType, body)
		default:
			http.Error(w, "Unsupported method: "+method, http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, "Error connecting to target: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy headers from target response
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Copy status code and body
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	log.Println("Listening on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// http_server.go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func StartHTTPServer() {
	http.HandleFunc("/clearcache", clearCacheHandler)
	http.HandleFunc("/reload", reloadHandler)

	addr := config.Server.HttpServer
	log.Printf("Starting HTTP server on %s\n", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Failed to start HTTP server: %s\n", err.Error())
	}
}

func clearCacheHandler(w http.ResponseWriter, r *http.Request) {
	СlearCache()
	fmt.Fprintln(w, "Cache cleared successfully")
}

func reloadHandler(w http.ResponseWriter, r *http.Request) {
	Rules = ParseConfig(rulesFile)
	fmt.Fprintln(w, "Rules reloaded successfully")
}

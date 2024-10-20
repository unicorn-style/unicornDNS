// http_server.go
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

func StartHTTPServer() {
	http.HandleFunc("/clearcache", clearCacheHandler)
	http.HandleFunc("/reload", reloadHandler)
	http.HandleFunc("/ips", capacityIPsHandler)
	http.HandleFunc("/rules", listRulesHandler)

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
	ParseRuleset(rulesFile)
	fmt.Fprintln(w, "Rules reloaded successfully")
}

func listRulesHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, PrintRules(Rules))
}

func capacityIPsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("TEST PAGE")

	type PoolStatus struct {
		Name   string
		IPList []IPEntry
	}

	var statusData []PoolStatus

	for name, pool := range ipv4Pools {
		mutex := mutexes[name]
		mutex.Lock()
		statusData = append(statusData, PoolStatus{Name: name, IPList: pool})
		mutex.Unlock()
	}

	/*
		for name, pool := range ipv6Pools {
			mutex := mutexes[name]
			mutex.Lock()
			statusData = append(statusData, PoolStatus{Name: name, IPList: pool})
			mutex.Unlock()
		}*/

	tmpl := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>IP Status</title>
		</head>
		<body>
			<h1>IP Status</h1>
			{{range .}}
				<h2>{{.Name}}</h2>
				<table border="1">
					<tr>
						<th>IP</th>
						<th>Status</th>
					</tr>
					{{range .IPList}}
						<tr>
							<td>{{.IP}}</td>
							<td>{{if .InUse}}In Use{{else}}Available{{end}}</td>
						</tr>
					{{end}}
				</table>
			{{end}}
		</body>
		</html>
	`

	t, err := template.New("status").Parse(tmpl)
	if err != nil {
		http.Error(w, "Error generating page", http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, statusData)
	if err != nil {
		http.Error(w, "Error generating page", http.StatusInternalServerError)
	}
}

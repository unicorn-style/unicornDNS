package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var (
	configFile string
	rulesFile  string
)

func init() {
	// cmd flags
	flag.StringVar(&configFile, "config", "config.yaml", "Path to the configuration file")
	flag.StringVar(&rulesFile, "rules", "rules.list", "Path to the rules file")
}

func main() {
	// Parce input CMD
	flag.Parse()

	// Loading config
	loadConfig(configFile)

	ParseRuleset(rulesFile) // Load rules slice
	log.Println("Rules: ", Cnt["RulesAll"].Get())
	log.Println("...accepted: ", Cnt["RulesActive"].Get())
	//log.Println(config.Actions["PROXY"])
	//log.Println(PrintRules(Rules))
	go StartHTTPServer()

	//cacheChan = make(chan cacheRequest)
	go cacheHandler()
	go startExpiryChecker()

	// Обработка системных сигналов для корректного завершения работы
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	dns.HandleFunc(".", handleDNSRequest)

	server := &dns.Server{Addr: config.Server.BindAddress, Net: "udp"}
	server.UDPSize = 1232
	log.Printf("Starting DNS server on %s\n", config.Server.BindAddress)

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatalf("Failed to start server: %s\n", err.Error())
		}
	}()

	// Exit
	sig := <-sigChan
	log.Printf("Received signal: %s. Shutting down...", sig)

	// Вызов функции очистки кэша
	СlearCache()

	// Остановка сервера
	server.Shutdown()

	// Отчистка всех правил
	ScriptResetAll()

	log.Println("Server gracefully stopped")
}

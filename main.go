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
	// Определяем флаги для файлов конфигурации
	flag.StringVar(&configFile, "config", "config.yaml", "Path to the configuration file")
	flag.StringVar(&rulesFile, "rules", "rules.txt", "Path to the rules file")
}

func main() {
	// Парсим аргументы командной строки
	flag.Parse()

	// Загружаем конфигурации и правила
	loadConfig(configFile)
	ParseConfig(rulesFile) // Загружаем правила в срез Rules
	go StartHTTPServer()

	//cacheChan = make(chan cacheRequest)
	go startExpiryChecker()
	go cacheHandler()

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

	// Ожидание сигнала завершения
	sig := <-sigChan
	log.Printf("Received signal: %s. Shutting down...", sig)

	// Вызов функции очистки кэша
	СlearCache()

	// Остановка сервера
	server.Shutdown()
	log.Println("Server gracefully stopped")
}

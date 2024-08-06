package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type CacheEntry struct {
	Domain   string
	Type     uint16
	RealIP   string
	TTL      uint32
	Expiry   time.Time
	LocalIPs []string
	CNAME    string // Добавлено поле для хранения CNAME записи
	Msg      *dns.Msg
}

var (
	cache      []CacheEntry
	cacheMutex sync.Mutex
	cacheChan  chan cacheRequest
)

type cacheRequest struct {
	Domain   string
	Type     uint16
	Response chan cacheResponse
}

type cacheResponse struct {
	Found   bool
	Entry   CacheEntry
	IsValid bool
	Msg     *dns.Msg
}

func cacheHandler() {
	for req := range cacheChan {
		var response cacheResponse
		cacheMutex.Lock()
		now := time.Now()
		newCache := cache[:0] // используем срез для очистки кэша от истекших записей
		//log.Printf("Req Cache %s", req.Domain)
		for _, entry := range cache {
			if entry.Expiry.After(now) {
				newCache = append(newCache, entry)
				trimmedEntryDomain := strings.TrimSuffix(entry.Domain, ".")
				//log.Println(req.Domain, entry.Domain, trimmedEntryDomain)
				if entry.Msg != nil && trimmedEntryDomain == req.Domain && entry.Type == req.Type {
					log.Println("CACHED")
					response = cacheResponse{Found: true, Msg: entry.Msg}
				}
			} else {
				removeRoute(req.Domain, entry.LocalIPs, entry.RealIP, entry.Type)
			}
		}
		cache = newCache
		cacheMutex.Unlock()
		req.Response <- response
	}
}

func СlearCache() {
	log.Println("DEL")
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	log.Println("DEL1")
	newCache := cache[:0] // используем срез для очистки кэша от истекших записей

	for _, entry := range cache {
		trimmedEntryDomain := strings.TrimSuffix(entry.Domain, ".")
		removeRoute(trimmedEntryDomain, entry.LocalIPs, entry.RealIP, entry.Type)
	}

	cache = newCache
}

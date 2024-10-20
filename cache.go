package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

/*var (
	cache      []cacheAnswer
	cacheMutex sync.Mutex
	cacheChan  = make(chan cacheTask)
)*/

type CacheEntry struct {
	Action       string
	Domain       string
	Type         uint16
	RealIP       string
	AllocatedIPs string
	TTL          uint32
	Expiry       time.Time
	Msg          []dns.RR
}

type cacheTask struct {
	Action   string
	Request  cacheRequest
	Response chan cacheResponse
}

type cacheRequest struct {
	Domain string
	Type   uint16
	Entry  CacheEntry
	Data   []CacheEntry
	Msg    *dns.Msg
}

type cacheAnswer struct {
	Domain string
	Type   uint16
	Action string
	Msg    *dns.Msg
	Entry  []CacheEntry
}

type cacheResponse struct {
	Found   bool
	Entry   CacheEntry
	IsValid bool
	Msg     *dns.Msg
}

var (
	cache      = make(map[string]cacheAnswer)
	cacheMutex sync.Mutex
	cacheChan  = make(chan cacheTask)
)

func cacheHandler() {
	for task := range cacheChan {
		cacheMutex.Lock()
		now := time.Now()

		// Очистка истекших записей в кэше
		for domain, answer := range cache {
			validEntries := []CacheEntry{}
			aCount := 0
			minTTL := time.Second * 5

			for _, entry := range answer.Entry {
				ttl := entry.Expiry.Sub(now).Seconds()
				var remainingTTL uint32
				if ttl > 0 {
					remainingTTL = uint32(ttl)
				} else {
					remainingTTL = 0
				}
				action, ok := config.Actions[entry.Action]
				if remainingTTL >= uint32(minTTL.Seconds()) {
					if len(entry.Msg) > 0 && entry.Msg[0].Header() != nil {
						sendTTL := remainingTTL
						if ok && sendTTL >= action.TTL.MaxTrasfer {
							sendTTL = action.TTL.MaxTrasfer
						}
						entry.Msg[0].Header().Ttl = sendTTL
					}
					if len(entry.Msg) > 0 && (entry.Msg[0].Header().Rrtype == dns.TypeA || entry.Msg[0].Header().Rrtype == dns.TypeAAAA) {
						aCount++
					}

					newEntry := CacheEntry{
						Action:       entry.Action,
						Domain:       entry.Domain,
						Type:         entry.Type,
						RealIP:       entry.RealIP,
						AllocatedIPs: entry.AllocatedIPs,
						TTL:          remainingTTL,
						Expiry:       entry.Expiry,
						Msg:          entry.Msg,
					}

					validEntries = append(validEntries, newEntry)
				}
			}

			if len(validEntries) > 0 && aCount > 0 {
				answer.Entry = validEntries
				cache[domain] = answer
			} else {
				delete(cache, domain)
			}
		}

		var response cacheResponse

		switch task.Action {
		case "Add":
			domain := strings.TrimSuffix(task.Request.Domain, ".")
			answer, found := cache[domain]

			if found && answer.Type == task.Request.Entry.Type {
				answer.Entry = append(answer.Entry, task.Request.Entry)
				cache[domain] = answer
			} else {
				cache[domain] = cacheAnswer{
					Domain: task.Request.Domain,
					Type:   task.Request.Type,
					Entry:  task.Request.Data,
					Msg:    task.Request.Msg,
				}
			}

			response = cacheResponse{Found: true, Entry: task.Request.Entry, IsValid: true}

		case "Search":
			domain := strings.TrimSuffix(task.Request.Domain, ".")
			answer, found := cache[domain]

			if found && answer.Type == task.Request.Type {
				foundEntries := []dns.RR{}
				foundAnswer := answer.Msg.Copy()

				for _, entry := range answer.Entry {
					if entry.Expiry.After(now) {

						if len(entry.Msg) > 0 {
							foundEntries = append(foundEntries, entry.Msg[0])
						}
					}
				}

				if len(foundEntries) > 0 {
					var combinedMsg dns.Msg
					combinedMsg.Answer = append(combinedMsg.Answer, foundEntries...)
					foundAnswer.Answer = combinedMsg.Answer

					response = cacheResponse{
						Found: true,
						Entry: CacheEntry{
							Domain: answer.Domain,
							Type:   task.Request.Type,
							Msg:    combinedMsg.Answer,
						},
						IsValid: true,
						Msg:     foundAnswer,
					}
				}
			}

		case "Delete":
			domain := strings.TrimSuffix(task.Request.Domain, ".")
			answer, found := cache[domain]

			if found && answer.Type == task.Request.Type {
				validEntries := []CacheEntry{}
				for _, entry := range answer.Entry {
					if !(strings.TrimSuffix(entry.Domain, ".") == task.Request.Domain && entry.Type == task.Request.Type) {
						validEntries = append(validEntries, entry)
					}
				}

				if len(validEntries) > 0 {
					answer.Entry = validEntries
					cache[domain] = answer
				} else {
					delete(cache, domain)
				}
			}
			response = cacheResponse{Found: false, IsValid: true}

		case "Reset":
			for ip := range ipMappings {
				removeRoute(ip)
			}
			cache = make(map[string]cacheAnswer)
			response = cacheResponse{Found: false, IsValid: true}
		}

		cacheMutex.Unlock()
		task.Response <- response
	}
}

func WriteCache(domains string, t uint16, ch []CacheEntry, msg *dns.Msg) {
	//fmt.Printf("111 %v\n", domains)
	writeCache := cacheRequest{
		Domain: domains,
		Type:   t,
		Data:   ch,
		Msg:    msg,
	}
	//fmt.Printf("WRITE t: %v\n", ch)
	cacheResp := make(chan cacheResponse)
	cacheChan <- cacheTask{Action: "Add", Request: writeCache, Response: cacheResp}
	<-cacheResp
}

func СlearCache() {
	log.Println("RESET ALL CACHE")
	cacheResp := make(chan cacheResponse)
	cacheChan <- cacheTask{Action: "RESET", Response: cacheResp}
	<-cacheResp
	log.Println("RESET DONE!")
}

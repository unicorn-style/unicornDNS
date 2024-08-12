package main

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var (
	cache      []cacheAnswer
	cacheMutex sync.Mutex
	cacheChan  = make(chan cacheTask)
)

type CacheEntry struct {
	Action       string
	Domain       string
	Type         uint16
	RealIP       string
	AllocatedIPs []string
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

func cacheHandler() {
	for task := range cacheChan {
		cacheMutex.Lock()
		now := time.Now()
		newCache := cache[:0] // используем срез для очистки кэша от истекших записей
		// Очистка истекших записей
		for _, answer := range cache {
			validEntries := []CacheEntry{}
			//expiredEntries := []CacheEntry{}
			aCount := 0
			minTTL := time.Second * 5 // Например, минимальный TTL - 5 секунд
			//fmt.Printf("---------- %v\n", answer.Entry)
			for _, entry := range answer.Entry {
				//remainingTTL := uint32(entry.Expiry.Sub(now).Seconds()) // Оставшийся TTL
				ttl := entry.Expiry.Sub(now).Seconds()
				var remainingTTL uint32
				if ttl > 0 {
					remainingTTL = uint32(ttl)
				} else {
					remainingTTL = 0
				}
				//fmt.Printf("-----------------------entry: %v [%v] %v\n", entry.Domain, entry.Expiry, len(entry.Msg.Answer))
				//fmt.Printf("TYPE %v | R: %v | S: %v\n", entry.Type, remainingTTL, uint32(minTTL.Seconds()))
				// Если оставшийся TTL больше минимального значения, обновляем запись и добавляем в валидные
				if remainingTTL >= uint32(minTTL.Seconds()) {
					// Создаем копию Msg для текущего CacheEntry
					if len(entry.Msg) > 0 && entry.Msg[0].Header() != nil {
						entry.Msg[0].Header().Ttl = remainingTTL
					}
					// Подсчет A/AAAA записей

					if (len(entry.Msg) > 0 && entry.Msg[0].Header() != nil) && (entry.Msg[0].Header().Rrtype == dns.TypeA || entry.Msg[0].Header().Rrtype == dns.TypeAAAA) {
						aCount++
					}
					//fmt.Printf("DATA \n\n\n%v\n\n\n", entry.Msg)

					// Обновляем запись с новым TTL
					newEntry := CacheEntry{
						Action:       entry.Action,
						Domain:       entry.Domain,
						Type:         entry.Type,
						RealIP:       entry.RealIP,
						AllocatedIPs: entry.AllocatedIPs,
						TTL:          remainingTTL,
						Expiry:       entry.Expiry,
						Msg:          entry.Msg, // Сообщение с обновленным TTL
					}

					validEntries = append(validEntries, newEntry)
				} /*else {
					expiredEntries = append(expiredEntries, entry)
				}*/

			}
			//log.Printf("CACHE STATE VALID: %v, A-LINES: %v", len(validEntries), aCount)
			// Если после проверки остались валидные записи и хотя бы одна из них A/AAAA
			if len(validEntries) > 0 && aCount > 0 {
				// Обновляем записи в ответе
				answer.Entry = validEntries
				newCache = append(newCache, answer)
			} /*else if len(expiredEntries) > 0 {
				// Если все записи истекли или нет A/AAAA записей, удаляем весь cacheAnswer
				// БЛОК под удаление
				go removeRoute(expiredEntries)
			} */
		}

		cache = newCache

		var response cacheResponse

		switch task.Action {
		case "Add":

			found := false
			for i, answer := range cache {
				trimmedEntryDomain := strings.TrimSuffix(answer.Domain, ".")
				if trimmedEntryDomain == task.Request.Domain && answer.Type == task.Request.Entry.Type {
					cache[i].Entry = append(cache[i].Entry, task.Request.Entry)
					found = true
					break
				}
			}
			if !found {
				cache = append(cache, cacheAnswer{
					Domain: task.Request.Domain,
					Type:   task.Request.Type,
					Entry:  task.Request.Data,
					Msg:    task.Request.Msg,
				})
			}
			//fmt.Printf("task.Request.Data: %v\n", len(task.Request.Data))
			response = cacheResponse{Found: true, Entry: task.Request.Entry, IsValid: true}

		case "Search":
			//fmt.Printf("!!!!!!!!!!!!!!!!!!cache:\n %v [%v]\n", task.Request.Domain, task.Request.Type)
			var foundEntries []dns.RR
			var foundAnswer *dns.Msg
			//fmt.Printf("111foundEntries: %v\n", foundEntries)
			var countT int16 = 0
			var countReq int16 = 0
			for _, answer := range cache {
				countT++
				trimmedEntryDomain := strings.TrimSuffix(task.Request.Domain, ".")
				//fmt.Printf("cache:\n %v == %v [%v == %v]\n", answer.Domain, task.Request.Domain, answer.Type, task.Request.Type)
				if answer.Domain == trimmedEntryDomain && answer.Type == task.Request.Type {
					foundAnswer = answer.Msg.Copy()
					//fmt.Printf("1cache:\n %v == %v [%v == %v]\n", answer.Domain, task.Request.Domain, answer.Type, task.Request.Type)
					for _, entry := range answer.Entry {
						//fmt.Println("CHECK", entry.Domain, entry.Type)
						countReq++
						if entry.Expiry.After(now) {
							//log.Println("CACHED", entry.Domain, entry.Type)
							//fmt.Printf("entry.Msg.Answer: %v\n", len(entry.Msg.Answer))
							//fmt.Printf("foundEntries: %v\n", foundEntries)
							if len(entry.Msg) > 0 {
								foundEntries = append(foundEntries, entry.Msg[0])
							}
						}
					}
				}
				//fmt.Printf("CountT: %v %v\n", countT, countReq)
				//fmt.Printf("CountReq: %v\n", countReq)
				//fmt.Printf("foundEntries: %v\n", foundEntries)
				if len(foundEntries) > 0 {
					// Создаем новое сообщение DNS с найденными записями
					var combinedMsg dns.Msg
					//fmt.Printf("ANWEREntries: %v\n", combinedMsg.Answer)
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
					break
				}
			}

		case "Delete":
			newCache := cache[:0]
			for _, answer := range cache {
				trimmedEntryDomain := strings.TrimSuffix(answer.Domain, ".")
				if !(trimmedEntryDomain == task.Request.Domain && answer.Type == task.Request.Type) {
					newCache = append(newCache, answer)
				} else {
					// Удаляем определённую запись CacheEntry из answer.Entry
					validEntries := []CacheEntry{}
					for _, entry := range answer.Entry {
						if !(strings.TrimSuffix(entry.Domain, ".") == task.Request.Domain && entry.Type == task.Request.Type) {
							validEntries = append(validEntries, entry)
						}
					}
					if len(validEntries) > 0 {
						answer.Entry = validEntries
						newCache = append(newCache, answer)
					} else {
						// Удаляем все записи
						removeRoute(answer.Entry)
					}
				}
			}
			cache = newCache
			response = cacheResponse{Found: false, IsValid: true}

		case "Reset":
			// Удаляем все записи из кэша и маршруты
			type reset struct {
				Entry  CacheEntry
				Action string
			}
			var resetEntries []reset

			for _, answer := range cache {
				for _, entry := range answer.Entry {
					resetEntries = append(resetEntries, reset{
						Entry:  entry,
						Action: answer.Action, // Используем Action, связанный с ответом
					})
				}
			}

			if len(resetEntries) > 0 {
				for _, entry := range resetEntries {
					removeRoute([]CacheEntry{entry.Entry})
				}
			}

			// Полностью очищаем кэш
			cache = []cacheAnswer{}
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

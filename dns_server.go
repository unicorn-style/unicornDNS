package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type IPEntry struct {
	IP    string
	InUse bool
}

type IPMapping struct {
	Domain     string
	RealIP     string
	LocalIPs   []string
	Action     string
	Expiry     time.Time
	RecordType string
}

var ipMappings = make(map[string]*IPMapping)
var mu sync.Mutex

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	//log.Printf("Received DNS request from %s\n", w.RemoteAddr().String())
	//log.Printf("DNS Request:\n%s\n", r.String())

	dnssecOK := false
	for _, extra := range r.Extra {
		if opt, ok := extra.(*dns.OPT); ok {
			if opt.Do() {
				dnssecOK = true
				break
			}
		}
	}
	// Проверяем, поддерживает ли клиент DNSSEC
	if r.RecursionDesired {
		msg.RecursionAvailable = true
	}

	msg.AuthenticatedData = true

	for _, question := range r.Question {
		handleQuestion(w, r, &msg, &question, dnssecOK)
	}
}

func handleQuestion(w dns.ResponseWriter, r *dns.Msg, msg *dns.Msg, question *dns.Question, dnssec bool) {
	domain := strings.TrimSuffix(question.Name, ".")
	log.Printf("Received query for: [%s], type: []\n", domain)
	//NewMsg := new(dns.Msg)
	//NewMsg.Question = msg.Copy().Question
	//fmt.Printf("r.Response: %v\n", r)

	// Passthrought any request not AAAA/A/
	/*if question.Qtype != dns.TypeA && question.Qtype != dns.TypeAAAA && question.Qtype != dns.TypeCNAME {
		log.Printf("Forwarding query for %s, type: %d to upstream server\n", domain, question.Qtype)
		ForwardAndRespond(w, r, msg, config.Server.DNSForward)
		return
	}*/

	//Request to cache
	searchRequest := cacheRequest{Domain: question.Name, Type: question.Qtype}
	cacheResp := make(chan cacheResponse)
	cacheChan <- cacheTask{Action: "Search", Request: searchRequest, Response: cacheResp}
	resp := <-cacheResp

	if resp.Found {
		//CACHE FOUND
		log.Printf("START Cache hit for %s %s %v\n", domain, msg.Extra, len(msg.Answer))
		msg.Answer = append(msg.Answer, resp.Entry.Msg...)
		//log.Printf("CACHE num %v\n", len(msg.Answer))
		//log.Printf("CACHE { \n\n\n\n\n %v\n\n\n\n }", resp.Entry.Msg)
		w.WriteMsg(msg)
		//log.Printf("END CACHE HIT %s %s\n", domain, msg.Extra)
		return
	}

	rule, matched := MatchRules(domain, Rules)
	var prepareCache []CacheEntry
	if matched {
		actionName := rule.Action
		_, ok := config.Actions[actionName]
		if ok {
			cDNS := config.Actions[actionName].DNSForward
			if len(cDNS) == 0 {
				cDNS = config.Server.DNSForward
			}
			log.Printf("Match: %s, DNS: %s [%s]\n", rule.Type, cDNS, actionName)
			records, ns, extra := forwardDNSRequest(r, config.Server.DNSForward, dnssec)
			msg.Ns = append(msg.Ns, ns...)
			msg.Extra = append(msg.Extra, extra...)

			var countAnswer uint32 = 0
			//var Newttl uint32
			for _, record := range records {
				if record.Header().Ttl < config.Actions[actionName].TTLMinRewrite {
					record.Header().Ttl = config.Actions[actionName].TTLMinRewrite
					//newttl = record.Header().Ttl
				}
				countAnswer++
				switch record := record.(type) {
				case *dns.A:
					prepareCache = append(prepareCache, handleARecord(record.Header().Name, record.A.String(), record.Header().Ttl, actionName, msg))
				case *dns.AAAA:
					prepareCache = append(prepareCache, handleAAAARecord(record.Header().Name, record.AAAA.String(), record.Header().Ttl, actionName, msg))
				case *dns.CNAME:
					prepareCache = append(prepareCache, handleCNAMERecord(record.Header().Name, record.Target, record.Header().Ttl, msg))
				default:
					log.Printf("RULE DNS: %s, DNS: %s [%s]\n", record.String(), cDNS, actionName)
					// Добавляем оригинальную запись в msg.Answer
					msg.Answer = append(msg.Answer, record)
					var Line []dns.RR
					Line = append(Line, record)

					// Добавляем в кеш оригинальную запись
					prepareCache = append(prepareCache, CacheEntry{
						Domain:       domain,
						Type:         record.Header().Rrtype,
						AllocatedIPs: nil,
						Expiry:       time.Now().Add(time.Duration(record.Header().Ttl) * time.Second),
						Msg:          Line,
					})
				}

			}
		} else {
			ForwardAndRespond(w, r, msg, config.Server.DNSForward)
		}
	} else {
		records, ns, extra := forwardDNSRequest(r, config.Server.DNSForward, dnssec)
		msg.Ns = append(msg.Ns, ns...)
		msg.Extra = append(msg.Extra, extra...)
		for _, record := range records {
			switch record := record.(type) {
			case *dns.A:
				prepareCache = append(prepareCache, handleARecord(record.Header().Name, record.A.String(), record.Header().Ttl, "", msg))
			case *dns.AAAA:
				prepareCache = append(prepareCache, handleAAAARecord(record.Header().Name, record.AAAA.String(), record.Header().Ttl, "", msg))
			case *dns.CNAME:
				prepareCache = append(prepareCache, handleCNAMERecord(record.Header().Name, record.Target, record.Header().Ttl, msg))
			default:
				log.Printf("DIRECT DNS\n")
				// Добавляем оригинальную запись в msg.Answer
				msg.Answer = append(msg.Answer, record)
				var Line []dns.RR
				Line = append(Line, record)
				// Добавляем в кеш оригинальную запись
				prepareCache = append(prepareCache, CacheEntry{
					Domain:       domain,
					Type:         record.Header().Rrtype, // Устанавливаем тип записи
					AllocatedIPs: nil,                    // Для не A/AAAA записей AllocatedIPs не нужны
					Expiry:       time.Now().Add(time.Duration(record.Header().Ttl) * time.Second),
					Msg:          Line,
				})
			}
		}

	}
	//log.Printf("DIRECT DNS: %s %v\n", domain, question.Qtype, prepareCache)
	WriteCache(domain, question.Qtype, prepareCache, msg.Copy())
	w.WriteMsg(msg)
}

func handleARecord(name, ip string, ttl uint32, actionName string, msg *dns.Msg) CacheEntry {
	log.Printf("Handling A record for %s\n", name)
	_, ok := config.Actions[actionName]
	if actionName != "" && ok {
		allocatedIPs := allocateIPs(config.Actions[actionName].IPv4Lists, ipv4Pools)
		if len(allocatedIPs) > 0 {
			//log.Printf("ADDROUTE: %s - %s\n", allocatedIPs, ip)
			allocatedIPs = addRoute(name, "A", allocatedIPs, ip, ttl, actionName)
			var Line []dns.RR
			Line = append(Line, createARecord(name, allocatedIPs[0], ttl))
			msg.Answer = append(msg.Answer, Line...)
			expiry := time.Now().Add(time.Duration(ttl) * time.Second) // Вычисляем время конца
			prepareCache := CacheEntry{
				Action:       actionName,
				Domain:       name,
				Type:         dns.TypeA,
				RealIP:       ip,
				AllocatedIPs: allocatedIPs,
				Expiry:       expiry,
				Msg:          Line,
			}
			return prepareCache
		}
		return CacheEntry{}
		//fmt.Printf("msg.Copy(): %v\n", msg.Copy())
	} else {
		log.Printf("Passthrought: %s, DNS: %s\n", name, config.Server.DNSForward)
		msg.Answer = append(msg.Answer, createARecord(name, ip, ttl))
		return CacheEntry{}
	}
}

func handleAAAARecord(name, ip string, ttl uint32, actionName string, msg *dns.Msg) CacheEntry {
	log.Printf("Handling AAAA record for %s\n", name)
	_, ok := config.Actions[actionName]
	if actionName != "" && ok {
		allocatedIPs := allocateIPs(config.Actions[actionName].IPv6Lists, ipv6Pools)
		if len(allocatedIPs) > 0 {
			//log.Printf("ADDROUTE: %s - %s\n", allocatedIPs, ip)
			allocatedIPs = addRoute(name, "AAAA", allocatedIPs, ip, ttl, actionName)
			var Line []dns.RR
			Line = append(Line, createAAAARecord(name, allocatedIPs[0], ttl))
			msg.Answer = append(msg.Answer, Line...)
			expiry := time.Now().Add(time.Duration(ttl) * time.Second) // Вычисляем время конца
			prepareCache := CacheEntry{
				Action:       actionName,
				Domain:       name,
				Type:         dns.TypeAAAA,
				RealIP:       ip,
				AllocatedIPs: allocatedIPs,
				Expiry:       expiry,
				Msg:          Line,
			}
			return prepareCache
		}
		return CacheEntry{}
	} else {
		log.Printf("Passthrought: %s, DNS: %s\n", name, config.Server.DNSForward)
		msg.Answer = append(msg.Answer, createAAAARecord(name, ip, ttl))
		return CacheEntry{}
	}
}

func handleCNAMERecord(name, target string, ttl uint32, msg *dns.Msg) CacheEntry {
	//log.Printf("Handling CNAME record for %s\n", name)

	var Line []dns.RR
	Line = append(Line, createCNAMERecord(name, target, ttl))
	msg.Answer = append(msg.Answer, Line...)
	//msg.Answer = append(msg.Answer, createCNAMERecord(name, target, ttl))
	expiry := time.Now().Add(time.Duration(ttl) * time.Second) // Вычисляем время конца
	prepareCache := CacheEntry{
		Domain: name,
		Type:   dns.TypeCNAME,
		RealIP: target,
		Expiry: expiry,
		Msg:    Line,
	}
	return prepareCache
}

func createARecord(name, ip string, ttl uint32) dns.RR {
	rr, _ := dns.NewRR(fmt.Sprintf("%s %d IN A %s", name, ttl, ip))
	return rr
}

func createAAAARecord(name, ip string, ttl uint32) dns.RR {
	rr, _ := dns.NewRR(fmt.Sprintf("%s %d IN AAAA %s", name, ttl, ip))
	return rr
}

func createCNAMERecord(name, target string, ttl uint32) dns.RR {
	rr, _ := dns.NewRR(fmt.Sprintf("%s %d IN CNAME %s", name, ttl, target))
	return rr
}

// Action CREATE
func addRoute(domain, recordType string, localIPs []string, realIP string, ttl uint32, actionName string) []string {
	log.Printf("Adding rule for %s of type %s with local IPs %v and real IP %s and TTL %d\n", domain, recordType, localIPs, realIP, ttl)

	action, ok := config.Actions[actionName]
	if !ok {
		log.Printf("No action found for %s", actionName)
		return nil
	}

	// Проверяем, существует ли уже запись для данного RealIP
	if mapping, exists := ipMappings[realIP]; exists {
		if mapping.Action == actionName {
			newExpiry := time.Now().Add(time.Duration(ttl) * time.Second)

			// Если новое правило имеет большее TTL, обновляем его
			if newExpiry.After(mapping.Expiry) {
				mu.Lock()
				mapping.Expiry = newExpiry
				mu.Unlock()
				log.Printf("[1]New TTL for %s -> %v", mapping.RealIP, mapping.LocalIPs)
				log.Printf("Checked TTL")
			}
			for _, ip := range localIPs {
				rtype := dns.TypeAAAA
				if recordType == "A" {
					rtype = dns.TypeA
				}
				releaseIP(ip, rtype)
				log.Printf("REALIZING %v %v", ip, recordType)
			}
			return mapping.LocalIPs
		}
	}

	// Добавляем новое сопоставление
	expiry := time.Now().Add(time.Duration(ttl) * time.Second)
	ipMappings[realIP] = &IPMapping{
		Domain:     domain,
		RealIP:     realIP,
		LocalIPs:   localIPs,
		Expiry:     expiry,
		Action:     actionName,
		RecordType: recordType,
	}

	var rule string
	if recordType == "A" {
		rule = action.IPv4Add
		for i, ip := range localIPs {
			rule = strings.ReplaceAll(rule, fmt.Sprintf("{ipv4%d}", i), ip)
		}
		rule = strings.ReplaceAll(rule, "{realIP}", realIP)
		rule = strings.ReplaceAll(rule, "{mark}", action.Mark)
	} else {
		rule = action.IPv6Add
		for i, ip := range localIPs {
			rule = strings.ReplaceAll(rule, fmt.Sprintf("{ipv6%d}", i), ip)
		}
		rule = strings.ReplaceAll(rule, "{realIP}", realIP)
		rule = strings.ReplaceAll(rule, "{mark}", action.Mark)
	}

	exec.Command("sh", "-c", rule).Run()
	return localIPs
}

// Action REMOVE
func removeRoute(entries []CacheEntry) {
	for _, entry := range entries {
		domain := entry.Domain
		realIP := entry.RealIP
		recordType := entry.Type
		localIPs := entry.AllocatedIPs
		actionString := entry.Action

		action, ok := config.Actions[actionString]
		if ok {
			log.Printf("Delete rule for %s [%v] %v", domain, actionString, localIPs)
			//fmt.Printf("action.IPv4Add: %v\n", action.IPv4Add[0])
			var rule string

			if recordType == dns.TypeA {
				rule = action.IPv4Delete
				for i, ip := range localIPs {
					rule = strings.ReplaceAll(rule, fmt.Sprintf("{ipv4%d}", i), ip)
				}
				rule = strings.ReplaceAll(rule, "{realIP}", realIP)
				rule = strings.ReplaceAll(rule, "{mark}", action.Mark)
				rule = strings.ReplaceAll(rule, "{domain}", domain)
			} else {
				rule = action.IPv6Delete
				for i, ip := range localIPs {
					rule = strings.ReplaceAll(rule, fmt.Sprintf("{ipv6%d}", i), ip)
				}
				rule = strings.ReplaceAll(rule, "{realIP}", realIP)
				rule = strings.ReplaceAll(rule, "{mark}", action.Mark)
				rule = strings.ReplaceAll(rule, "{domain}", domain)
			}

			log.Printf("SH SCRIPT [%v]", rule)
			cmd := exec.Command("sh", "-c", rule).Run()
			log.Printf("RESULT [%v]", cmd)

			for _, ip := range localIPs {
				releaseIP(ip, recordType)
				log.Printf("REALIZE %v %v", ip, recordType)
			}
		} /* else {
			log.Printf("removeRoute, No action found for [domain: %v] D: %v -> %v\n", domain, realIP, localIPs)
		} */
	}
}
func forwardDNSRequest(r *dns.Msg, dnsForward string, dnssec bool) ([]dns.RR, []dns.RR, []dns.RR) {
	client := new(dns.Client)
	client.UDPSize = 1232 // увеличенный размер UDP для DNSSEC
	client.DialTimeout = time.Second * 2
	client.ReadTimeout = time.Second * 2

	// Клонируем оригинальный запрос
	req := r.Copy()

	// Если требуется, можно изменить параметры, например, установить размер EDNS0
	if edns0 := req.IsEdns0(); edns0 != nil {
		req.SetEdns0(edns0.UDPSize(), edns0.Do())
	} else {
		req.SetEdns0(1232, dnssec)
	}

	// Выполняем запрос к форвардному DNS-серверу
	response, _, err := client.Exchange(req, dnsForward)
	if err != nil {
		log.Printf("Failed to forward DNS request: %s\n", err)
		return nil, nil, nil
	}

	//debug answer
	//fmt.Printf("response: %v\n", response)

	return response.Answer, response.Ns, response.Extra
}

// Passthrought DNS query
func ForwardAndRespond(w dns.ResponseWriter, r *dns.Msg, msg *dns.Msg, dnsForward string) {
	client := new(dns.Client)
	client.UDPSize = 1232 // увеличенный размер UDP для DNSSEC
	client.DialTimeout = time.Second * 2
	client.ReadTimeout = time.Second * 2
	//client.UDPSize = 1232 // увеличенный размер UDP для DNSSEC
	log.Printf("Passthrough: %s, DNS: %s\n", &r.Question[0], config.Server.DNSForward)
	req := r.Copy()

	response, _, err := client.Exchange(req, dnsForward)

	if err != nil {
		log.Printf("Failed to forward DNS request: %s\n", err)
		w.WriteMsg(msg)
		return
	}
	msg.Answer = append(msg.Answer, response.Answer...)
	msg.Ns = append(msg.Ns, response.Ns...)
	msg.Extra = append(msg.Extra, response.Extra...)
	w.WriteMsg(msg)
}

func startExpiryChecker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		mu.Lock()
		for ip, mapping := range ipMappings {
			if now.After(mapping.Expiry) {
				rtype := dns.TypeAAAA
				if mapping.RecordType == "A" {
					rtype = dns.TypeA
				}
				removeRoute([]CacheEntry{{Domain: mapping.Domain, RealIP: ip, AllocatedIPs: mapping.LocalIPs, Action: mapping.Action, Type: rtype}})
				delete(ipMappings, ip)
				log.Printf("Expired mapping removed for IP: %s -> %s [%s]", ip, mapping.LocalIPs, mapping.Action)
			}
		}
		mu.Unlock()
	}
}

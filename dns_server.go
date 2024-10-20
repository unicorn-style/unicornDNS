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

var mu sync.Mutex

func expireTime(i uint32) time.Time {
	return time.Now().Add(time.Duration(i) * time.Second)
}

// Input DNS Request
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
	// Check DNSSEC & Recursion
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
			records, ns, extra := forwardDNSRequest(r, cDNS, dnssec)
			msg.Ns = append(msg.Ns, ns...)
			msg.Extra = append(msg.Extra, extra...)
			if config.Actions[actionName].Method == "fakeip" {
				var countAnswer uint32 = 0
				//var Newttl uint32
				for _, record := range records {
					//ttl constructor
					sendTTL := record.Header().Ttl
					//ttl.min-rewrite
					if record.Header().Ttl < config.Actions[actionName].TTL.MinRewrite {
						sendTTL = config.Actions[actionName].TTL.MinRewrite
					}
					if record.Header().Ttl > config.Actions[actionName].TTL.MaxRewrite {
						sendTTL = config.Actions[actionName].TTL.MaxRewrite
					}
					countAnswer++
					switch record := record.(type) {
					case *dns.A:
						prepareCache = append(prepareCache, handleARecord(record.Header().Name, record.A.String(), sendTTL, actionName, msg))
					case *dns.AAAA:
						prepareCache = append(prepareCache, handleAAAARecord(record.Header().Name, record.AAAA.String(), sendTTL, actionName, msg))
					case *dns.CNAME:
						prepareCache = append(prepareCache, handleCNAMERecord(record.Header().Name, record.Target, sendTTL, msg))
					default:
						log.Printf("RULE DNS: %s, DNS: %s [%s]\n", record.String(), cDNS, actionName)
						// Add original line to msg.Answer
						msg.Answer = append(msg.Answer, record)
						var Line []dns.RR
						Line = append(Line, record)

						// Добавляем в кеш оригинальную запись
						prepareCache = append(prepareCache, CacheEntry{
							Domain:       domain,
							Type:         record.Header().Rrtype,
							AllocatedIPs: "",
							Expiry:       expireTime(sendTTL),
							Msg:          Line,
						})
					}

				}
			} else {
				ForwardAndRespond(w, r, msg, cDNS)
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
					AllocatedIPs: "",                     // Для не A/AAAA записей AllocatedIPs не нужны
					Expiry:       expireTime(record.Header().Ttl),
					Msg:          Line,
				})
			}
		}

	}
	//log.Printf("DIRECT DNS: %s %v\n", domain, question.Qtype, prepareCache)
	WriteCache(domain, question.Qtype, prepareCache, msg.Copy())
	w.WriteMsg(msg)
}

// Handlers record
func handleARecord(name, ip string, ttl uint32, actionName string, msg *dns.Msg) CacheEntry {
	log.Printf("Handling A record for %s\n", name)
	_, ok := config.Actions[actionName]
	if actionName != "" && ok {
		allocatedIPs := addRoute(name, "A", ip, ttl, actionName)
		if len(allocatedIPs) > 0 {
			//log.Printf("ADDROUTE: %s - %s\n", allocatedIPs, ip)
			var Line []dns.RR
			//ttl.max-transfare
			sendTTL := ttl
			if ttl > config.Actions[actionName].TTL.MaxTrasfer {
				sendTTL = config.Actions[actionName].TTL.MaxTrasfer
			}
			Line = append(Line, createARecord(name, allocatedIPs, sendTTL))
			msg.Answer = append(msg.Answer, Line...)
			prepareCache := CacheEntry{
				Action:       actionName,
				Domain:       name,
				Type:         dns.TypeA,
				RealIP:       ip,
				AllocatedIPs: allocatedIPs,
				Expiry:       expireTime(ttl),
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
		allocatedIPs := addRoute(name, "AAAA", ip, ttl, actionName)
		if len(allocatedIPs) > 0 {
			//log.Printf("ADDROUTE: %s - %s\n", allocatedIPs, ip)

			var Line []dns.RR
			//Line = append(Line, createAAAARecord(name, allocatedIPs[0], ttl))
			//ttl.max-transfare
			sendTTL := ttl
			if ttl > config.Actions[actionName].TTL.MaxTrasfer {
				sendTTL = config.Actions[actionName].TTL.MaxTrasfer
			}
			Line = append(Line, createAAAARecord(name, allocatedIPs, sendTTL))
			msg.Answer = append(msg.Answer, Line...)
			prepareCache := CacheEntry{
				Action:       actionName,
				Domain:       name,
				Type:         dns.TypeAAAA,
				RealIP:       ip,
				AllocatedIPs: allocatedIPs,
				Expiry:       expireTime(ttl),
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
	var Line []dns.RR
	Line = append(Line, createCNAMERecord(name, target, ttl))
	msg.Answer = append(msg.Answer, Line...)
	//msg.Answer = append(msg.Answer, createCNAMERecord(name, target, ttl))
	//expiry := time.Now().Add(time.Duration(ttl) * time.Second) // Вычисляем время конца
	prepareCache := CacheEntry{
		Domain: name,
		Type:   dns.TypeCNAME,
		RealIP: target,
		Expiry: expireTime(ttl),
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

// Exec Add Rule
func addRoute(domain, recordType string, realIP string, ttl uint32, actionName string) string {
	log.Printf("Adding rule for %s of type %s real IP %s and TTL %d\n", domain, recordType, realIP, ttl)

	action, ok := config.Actions[actionName]
	if !ok {
		log.Printf("No action found for %s", actionName)
		return ""
	}
	eTime := ttl
	if action.FakeIPDelay != 0 {
		eTime = ttl + action.FakeIPDelay
	}

	newExpiry := expireTime(eTime)
	//fmt.Println("ttl", eTime)
	//fmt.Println("fakeIPDelay", action.FakeIPDelay)
	// Check lease, if exists - update expiry
	if mapping, exists := ipMappings[realIP]; exists {
		if mapping.Action == actionName {
			// Если новое правило имеет большее TTL, обновляем его
			if newExpiry.After(mapping.Expiry) {
				mu.Lock()
				mapping.Expiry = newExpiry
				mu.Unlock()
				log.Printf("[1]New TTL for %s -> %v", mapping.RealIP, mapping.LocalIPs)
				log.Printf("Checked TTL")
			}
			return mapping.LocalIPs
		}
	}
	var localIPs string
	var rule, ruleD string
	rule = action.Script.Add
	//log.Println("ADD:", rule)
	ruleD = action.Script.Delete
	itype := ""
	for _, NetName := range action.FakeIPNet {
		if recordType == "A" {
			itype = "4"
			localIPs = allocateIP(NetName, 4, ipv4Pools)
		}
		if recordType == "AAAA" {
			localIPs = allocateIP(NetName, 6, ipv6Pools)
			itype = "6"
		}
		ipMappings[realIP] = &IPMapping{
			Domain:     domain,
			RealIP:     realIP,
			LocalIPs:   localIPs,
			Expiry:     newExpiry,
			Action:     actionName,
			RecordType: recordType,
		}

		// Добавляем новое сопоставление

		rule = strings.ReplaceAll(rule, fmt.Sprintf("{fakeIP_%v}", NetName), localIPs)
		rule = strings.ReplaceAll(rule, "{realIP}", realIP)
		rule = strings.ReplaceAll(rule, "{inet}", itype)
		//delete rules
		ruleD = strings.ReplaceAll(ruleD, "{realIP}", realIP)
		ruleD = strings.ReplaceAll(ruleD, fmt.Sprintf("{fakeIP_%v}", NetName), localIPs)
		ruleD = strings.ReplaceAll(ruleD, "{inet}", itype)
		ipMappings[realIP].CmdDelete = ruleD
		log.Println("ADD:", rule)
		//log.Println("DEL:", ruleD)

		exec.Command("sh", "-c", rule).Run()
	}
	return localIPs
}

// Exec Remove Rule
func removeRoute(ip string) {
	_, ok := ipMappings[ip]
	if ok {
		log.Printf("Expired IP-local {%v} was released [%v - %v]", ip, ipMappings[ip].Domain, ipMappings[ip].RealIP)
		exec.Command("sh", "-c", ipMappings[ip].CmdDelete).Run()
		releaseIP(ipMappings[ip].LocalIPs)
	}
}
func forwardDNSRequest(r *dns.Msg, dnsForward string, dnssec bool) ([]dns.RR, []dns.RR, []dns.RR) {
	client := new(dns.Client)
	client.UDPSize = 1232 // увеличенный размер UDP для DNSSEC
	client.DialTimeout = time.Second * 2
	client.ReadTimeout = time.Second * 2

	// Dublicate
	req := r.Copy()

	// Если требуется, можно изменить параметры, например, установить размер EDNS0
	if edns0 := req.IsEdns0(); edns0 != nil {
		req.SetEdns0(edns0.UDPSize(), edns0.Do())
	} else {
		req.SetEdns0(1232, dnssec)
	}

	// forward to DNS-Server
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
	client.UDPSize = 1232 //  UDP for DNSSEC
	client.DialTimeout = time.Second * 2
	client.ReadTimeout = time.Second * 2

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
				log.Printf("EC Expired mapping for IP: %s -> %s [%s]", ip, mapping.LocalIPs, mapping.Action)
				removeRoute(mapping.RealIP)
				delete(ipMappings, mapping.RealIP)
				//delete(allocatedIPs, mapping.LocalIPs[0])
			}
		}
		mu.Unlock()
	}
}

// Run reset script by name
func ScriptOnReset(actionString string) {
	action, ok := config.Actions[actionString]
	var cmd error
	if ok {
		if len(action.Script.OnReset) > 0 {
			log.Printf("SH RUN SCRIPT ACTION (OnReset) [%v]", action.Script.OnReset)
			cmd = exec.Command("sh", "-c", action.Script.OnReset).Run()
			log.Printf("RESULT [%v]", cmd)
		}
	}
}

func ScriptOnStart(actionString string) {
	action, ok := config.Actions[actionString]
	var cmd error
	if ok {
		if len(action.Script.OnStart) > 0 {
			log.Printf("SH RUN SCRIPT ACTION (OnStart) [%v]", action.Script.OnStart)
			cmd = exec.Command("sh", "-c", action.Script.OnStart).Run()
			log.Printf("RESULT [%v]", cmd)
		}
	}
}

func ScriptResetAll() {
	for _, action := range config.Actions {
		if len(action.Script.OnReset) > 0 {
			log.Printf("SH RUN SCRIPT ACTION (OnReset) [%v]", action.Script.OnReset)
			exec.Command("sh", "-c", action.Script.OnReset).Run()
		}
	}
}

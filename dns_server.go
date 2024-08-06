package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type IPEntry struct {
	IP    string
	InUse bool
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	for _, question := range r.Question {
		handleQuestion(w, r, &msg, &question)
	}
}

func handleQuestion(w dns.ResponseWriter, r *dns.Msg, msg *dns.Msg, question *dns.Question) {
	domain := strings.TrimSuffix(question.Name, ".")
	log.Printf("Received query for: %s, type: %d\n", domain, question.Qtype)

	cacheResp := make(chan cacheResponse)
	cacheChan <- cacheRequest{Domain: domain, Type: question.Qtype, Response: cacheResp}
	resp := <-cacheResp

	if resp.Found {
		log.Printf("Cache hit for %s\n", domain)
		if resp.Msg != nil {
			msg.Answer = append(msg.Answer, resp.Msg.Answer...)
			w.WriteMsg(msg)
			return
		}
		w.WriteMsg(msg)
		return
	}

	rule, matched := MatchRules(domain, Rules)
	if matched {
		actionName := rule.Action
		cDNS := config.Actions[actionName].DNSForward
		if len(cDNS) == 0 {
			cDNS = config.Server.DNSForward
		}
		log.Printf("Match: %s, DNS: %s [%s]\n", rule.Type, cDNS, actionName)
		records := forwardDNSRequest(r, cDNS)
		for _, record := range records {
			if record.Header().Ttl < config.Actions[actionName].RewriteTTL {
				record.Header().Ttl = config.Actions[actionName].RewriteTTL
			}
			switch record := record.(type) {
			case *dns.A:
				handleARecord(question.Name, record.A.String(), record.Header().Ttl, actionName, msg)
			case *dns.AAAA:
				handleAAAARecord(question.Name, record.AAAA.String(), record.Header().Ttl, actionName, msg)
			case *dns.CNAME:
				handleCNAMERecord(question.Name, record.Target, record.Header().Ttl, msg)
			}
		}
	} else {
		log.Printf("Passthrough: %s, DNS: %s\n", domain, config.Server.DNSForward)
		records := forwardDNSRequest(r, config.Server.DNSForward)
		for _, record := range records {
			switch record := record.(type) {
			case *dns.A:
				handleARecord(question.Name, record.A.String(), record.Header().Ttl, "", msg)
			case *dns.AAAA:
				handleAAAARecord(question.Name, record.AAAA.String(), record.Header().Ttl, "", msg)
			case *dns.CNAME:
				handleCNAMERecord(question.Name, record.Target, record.Header().Ttl, msg)
			}
		}
	}
	w.WriteMsg(msg)
}

func handleARecord(name, ip string, ttl uint32, actionName string, msg *dns.Msg) {
	log.Printf("Handling A record for %s\n", name)
	_, ok := config.Actions[actionName]
	if actionName != "" && ok {
		allocatedIPs := allocateIPs(config.Actions[actionName].IPv4Lists, ipv4Pools)
		if len(allocatedIPs) > 0 {
			log.Printf("ADDROUTE: %s - %s\n", allocatedIPs, ip)
			addRoute(name, "A", allocatedIPs, ip, ttl, actionName)
			msg.Answer = append(msg.Answer, createARecord(name, allocatedIPs[0], ttl))

			cacheMutex.Lock()
			cache = append(cache, CacheEntry{Domain: name, Type: dns.TypeA, RealIP: ip, TTL: ttl, Expiry: time.Now().Add(time.Duration(ttl) * time.Second), LocalIPs: allocatedIPs, Msg: msg.Copy()})
			cacheMutex.Unlock()
		}
	} else {
		log.Printf("Passthrought: %s, DNS: %s\n", name, config.Server.DNSForward)
		msg.Answer = append(msg.Answer, createARecord(name, ip, ttl))
		cacheMutex.Lock()
		cache = append(cache, CacheEntry{RealIP: ip, TTL: ttl, Expiry: time.Now().Add(time.Duration(ttl) * time.Second), LocalIPs: []string{ip}})
		cacheMutex.Unlock()
	}
}

func handleAAAARecord(name, ip string, ttl uint32, actionName string, msg *dns.Msg) {
	log.Printf("Handling AAAA record for %s\n", name)
	allocatedIPs := allocateIPs(config.Actions[actionName].IPv6Lists, ipv6Pools)
	_, ok := config.Actions[actionName]
	if actionName != "" && ok {
		if len(allocatedIPs) > 0 {
			log.Printf("ADDROUTE: %s - %s\n", allocatedIPs, ip)
			addRoute(name, "AAAA", allocatedIPs, ip, ttl, actionName)
			msg.Answer = append(msg.Answer, createAAAARecord(name, allocatedIPs[0], ttl))

			cacheMutex.Lock()
			cache = append(cache, CacheEntry{Domain: name, Type: dns.TypeAAAA, RealIP: ip, TTL: ttl, Expiry: time.Now().Add(time.Duration(ttl) * time.Second), LocalIPs: allocatedIPs, Msg: msg.Copy()})
			cacheMutex.Unlock()
		}
	} else {
		log.Printf("Passthrought: %s, DNS: %s\n", name, config.Server.DNSForward)
		msg.Answer = append(msg.Answer, createAAAARecord(name, ip, ttl))
		cacheMutex.Lock()
		cache = append(cache, CacheEntry{RealIP: ip, TTL: ttl, Expiry: time.Now().Add(time.Duration(ttl) * time.Second), LocalIPs: []string{ip}})
		cacheMutex.Unlock()
	}
}

func handleCNAMERecord(name, target string, ttl uint32, msg *dns.Msg) {
	log.Printf("Handling CNAME record for %s\n", name)
	msg.Answer = append(msg.Answer, createCNAMERecord(name, target, ttl))

	cacheMutex.Lock()
	cache = append(cache, CacheEntry{Domain: name, Type: dns.TypeCNAME, CNAME: target, TTL: ttl, Expiry: time.Now().Add(time.Duration(ttl) * time.Second), Msg: msg.Copy()})
	cacheMutex.Unlock()
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

func addRoute(domain, recordType string, localIPs []string, realIP string, ttl uint32, actionName string) {
	log.Printf("Adding rule for %s of type %s with local IPs %v and real IP %s and TTL %d\n", domain, recordType, localIPs, realIP, ttl)

	action, ok := config.Actions[actionName]
	if !ok {
		log.Fatalf("No action found for %s", actionName)
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
}

func removeRoute(domain string, localIPs []string, realIP string, recordType uint16) {
	log.Printf("Delete rule for %s", domain)
	action, ok := config.Actions["PROXY"]
	if !ok {
		log.Fatalf("No action found for PROXY")
	}

	var rule string

	if recordType == dns.TypeA {
		rule = action.IPv4Delete
		for i, ip := range localIPs {
			rule = strings.ReplaceAll(rule, fmt.Sprintf("{ipv4%d}", i), ip)
		}
		rule = strings.ReplaceAll(rule, "{realIP}", realIP)
		rule = strings.ReplaceAll(rule, "{mark}", action.Mark)
	} else {
		rule = action.IPv6Delete
		for i, ip := range localIPs {
			rule = strings.ReplaceAll(rule, fmt.Sprintf("{ipv6%d}", i), ip)
		}
		rule = strings.ReplaceAll(rule, "{realIP}", realIP)
		rule = strings.ReplaceAll(rule, "{mark}", action.Mark)
	}

	exec.Command("sh", "-c", rule).Run()

	for _, ip := range localIPs {
		releaseIP(ip, recordType)
	}
}

func forwardDNSRequest(r *dns.Msg, dnsForward string) []dns.RR {
	client := new(dns.Client)
	response, _, err := client.Exchange(r, dnsForward)
	if err != nil {
		log.Printf("Failed to forward DNS request: %s\n", err)
		return nil
	}

	return response.Answer
}

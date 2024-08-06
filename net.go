package main

import (
	"log"

	"github.com/miekg/dns"
)

func allocateIPs(listNames []string, pools map[string][]IPEntry) []string {
	var allocatedIPs []string
	for _, listName := range listNames {
		mutex, ok := mutexes[listName]
		if !ok {
			log.Fatalf("No mutex found for list %s", listName)
		}
		pool, ok := pools[listName]
		if !ok {
			log.Fatalf("No pool found for list %s", listName)
		}

		mutex.Lock()
		for i := range pool {
			if !pool[i].InUse {
				pool[i].InUse = true
				allocatedIPs = append(allocatedIPs, pool[i].IP)
				break
			}
		}
		mutex.Unlock()
	}
	return allocatedIPs
}

func releaseIP(ip string, recordType uint16) {
	var pools map[string][]IPEntry
	if recordType == dns.TypeA {
		pools = ipv4Pools
	} else {
		pools = ipv6Pools
	}

	for name, pool := range pools {
		mutex := mutexes[name]
		mutex.Lock()
		for i := range pool {
			if pool[i].IP == ip {
				pool[i].InUse = false
				break
			}
		}
		mutex.Unlock()
	}
}

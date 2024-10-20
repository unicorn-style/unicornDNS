package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type IPMapping struct {
	Domain     string
	RealIP     string
	LocalIPs   string
	Action     string
	Expiry     time.Time
	RecordType string
	CmdDelete  string
}

var ipMappings = make(map[string]*IPMapping)

type IPEntry struct {
	IP       string
	ListName string // Имя пула, из которого выделен IP
}

var allocatedIPs = make(map[string]*IPEntry)
var allocatedMutex *sync.Mutex

func allocateIP(listName string, inet int8, pools map[string][]IPEntry) string {
	var allocatedIPsList string
	mutex, ok := mutexes[listName]
	if !ok {
		log.Fatalf("No mutex found for list %s", listName)
	}
	pool, ok := pools[listName]
	if !ok {
		log.Fatalf("No pool found for list %s", listName)
	}
	//if len(net.IPv4) > 0 && inet == 4 {
	mutex.Lock()
	for i := range pool {
		if _, ok := allocatedIPs[pool[i].IP]; !ok {
			allocatedIPsList = pool[i].IP
			allocatedIPs[pool[i].IP] = &pool[i] // Сохраняем ссылку на выделенный IP
			fmt.Println(allocatedIPs[pool[i].IP])
			break
		}
	}
	log.Printf("%v\n", inet)
	mutex.Unlock()
	//}
	/*if len(net.IPv6) > 0 && inet == 6 {
		mutex.Lock()
		for i := range pool {
			if _, ok := allocatedIPs[pool[i].IP]; !ok {
				allocatedIPsList = pool[i].IP
				allocatedIPs[pool[i].IP] = &pool[i] // Сохраняем ссылку на выделенный IP
				fmt.Println(allocatedIPs[pool[i].IP])
				break
			}
		}
		mutex.Unlock()
	}*/
	return allocatedIPsList
}

func releaseIP(ip string) {
	allocatedMutex.Lock()
	defer allocatedMutex.Unlock()
	_, ok := allocatedIPs[ip]
	if ok {
		delete(allocatedIPs, ip) // Удаляем из мапы при освобождении
	} else {
		log.Printf("IP %s not found in allocated IPs", ip)
	}
}

func generateIPPool(cidr string) []IPEntry {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatalf("Failed to parse CIDR: %s\n", err)
	}

	var pool []IPEntry
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
		pool = append(pool, IPEntry{IP: ip.String()})
	}

	// Removing network and broadcast addresses for IPv4
	if ipnet.IP.To4() != nil {
		return pool[1 : len(pool)-1]
	}
	return pool
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

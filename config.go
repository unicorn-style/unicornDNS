package main

import (
	"log"
	"net"
	"os"
	"regexp"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		BindAddress string `yaml:"bind-address"`
		HttpServer  string `yaml:"http-server"`
		DNSForward  string `yaml:"dns-forward"`
	} `yaml:"server"`
	Networks map[string]Network `yaml:"networks"`
	Actions  map[string]Action  `yaml:"actions"`
}

type Network struct {
	CIDR string `yaml:"cidr"`
}

type Action struct {
	Mark       string   `yaml:"mark"`
	DNSForward string   `yaml:"dns-forward"`
	RewriteTTL uint32   `yaml:"ttl-rewrite"`
	IPv4Lists  []string `yaml:"ipv4-lists"`
	IPv6Lists  []string `yaml:"ipv6-lists"`
	IPv4Add    string   `yaml:"ipv4-run-add"`
	IPv4Delete string   `yaml:"ipv4-run-delete"`
	IPv4Reset  string   `yaml:"ipv4-run-reset"`
	IPv6Add    string   `yaml:"ipv6-run-add"`
	IPv6Delete string   `yaml:"ipv6-run-delete"`
	IPv6Reset  string   `yaml:"ipv6-run-reset"`
}

var (
	config      Config
	ipv4Pools   map[string][]IPEntry
	ipv6Pools   map[string][]IPEntry
	mutexes     map[string]*sync.Mutex
	validNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

func loadConfig(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	ipv4Pools = make(map[string][]IPEntry)
	ipv6Pools = make(map[string][]IPEntry)
	mutexes = make(map[string]*sync.Mutex)

	for name, network := range config.Networks {
		if !validNameRe.MatchString(name) {
			log.Fatalf("Invalid network name: %s", name)
		}
		log.Printf("CONFIG NET %s", name)
		ipv4Pools[name] = generateIPPool(network.CIDR)
		ipv6Pools[name] = generateIPPool(network.CIDR)
		mutexes[name] = &sync.Mutex{}
	}
}

func generateIPPool(cidr string) []IPEntry {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatalf("Failed to parse CIDR: %s\n", err)
	}

	var pool []IPEntry
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {
		pool = append(pool, IPEntry{IP: ip.String(), InUse: false})
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

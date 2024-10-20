package main

import (
	"log"
	"os"
	"regexp"
	"strings"
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
	IPv4 string `yaml:"ipv4"`
	IPv6 string `yaml:"ipv6"`
}

type Action struct {
	Mark        string            `yaml:"mark"`
	DNSForward  string            `yaml:"dns-forward"`
	Method      string            `yaml:"method"`
	FakeIPDelay uint32            `yaml:"fakeip-release-delay"`
	Variable    map[string]string `yaml:"variable"`
	TTL         TTL               `yaml:"ttl"`
	FakeIPNet   []string          `yaml:"fakeip-networks"`
	Script      Scripts           `yaml:"script"`
}

type Scripts struct {
	Add     string `yaml:"add"`
	Delete  string `yaml:"delete"`
	OnStart string `yaml:"onstart"`
	OnReset string `yaml:"onreset"`
}
type TTL struct {
	MaxTrasfer uint32 `yaml:"max-transfer"`
	MinRewrite uint32 `yaml:"min-rewrite"`
	MaxRewrite uint32 `yaml:"max-rewrite"`
}

var (
	config      Config
	ipv4Pools   map[string][]IPEntry
	ipv6Pools   map[string][]IPEntry
	mutexes     map[string]*sync.Mutex
	validNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

func replaceVariables(text string, variables map[string]string) string {
	for key, value := range variables {
		// Меняем все вхождения переменных вида {key} на их значение
		placeholder := "{" + key + "}"
		text = strings.ReplaceAll(text, placeholder, value)
	}
	//log.Printf(text)
	return text
}

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
	allocatedMutex = &sync.Mutex{}

	for name, network := range config.Networks {
		if !validNameRe.MatchString(name) {
			log.Fatalf("Invalid network name: %s", name)
		}
		log.Printf("CONFIG NET %s", name)
		if len(network.IPv4) > 0 {
			ipv4Pools[name] = generateIPPool(network.IPv4)
		}
		if len(network.IPv6) > 0 {
			ipv6Pools[name] = generateIPPool(network.IPv6)
		}
		mutexes[name] = &sync.Mutex{}
	}

	// Проходим по всем действиям и применяем замену переменных
	for actionName, action := range config.Actions {
		if len(action.Variable) > 0 {
			action.Script.Add = replaceVariables(action.Script.Add, action.Variable)
			action.Script.Delete = replaceVariables(action.Script.Delete, action.Variable)
			action.Script.OnStart = replaceVariables(action.Script.OnStart, action.Variable)
			action.Script.OnReset = replaceVariables(action.Script.OnReset, action.Variable)
		}
		config.Actions[actionName] = action
		log.Printf("Action %s processed", actionName)
		ScriptOnStart(actionName)
	}
}

package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type Rule struct {
	Type        string
	Text        string
	Passthrough bool
	TTL         int32
	Action      string
	DNS         string
}

var Rules []Rule

var rulemu sync.RWMutex

func ParseConfig(filename string) {
	rulemu.Lock()
	defer rulemu.Unlock()

	Rules = []Rule{}
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "RULE-SET") {
			Rules = append(Rules, parseRuleSet(line)...)
		} else {
			rule := parseLine(line)
			if rule != (Rule{}) {
				Rules = append(Rules, rule)
			}
		}
	}
}

func parseLine(line string) Rule {
	parts := strings.Split(line, ",")
	if len(parts) < 3 {
		return Rule{}
	}

	rule := Rule{
		Type:   parts[0],
		Text:   parts[1],
		Action: parts[2],
	}

	// Обработка дополнительных аргументов
	for _, part := range parts[3:] {
		if part == "passthrought" {
			rule.Passthrough = true
		} else if strings.HasPrefix(part, "ttl=") {
			fmt.Sscanf(part, "ttl=%d", &rule.TTL)
		} else if strings.HasPrefix(part, "dns=") {
			rule.DNS = strings.TrimPrefix(part, "dns=")
		}
	}

	// Добавляем только правила типа DOMAIN-SUFFIX и DOMAIN-KEYWORD
	if rule.Type == "DOMAIN-SUFFIX" || rule.Type == "DOMAIN-KEYWORD" {
		return rule
	}

	return Rule{}
}

func parseRuleSet(line string) []Rule {
	parts := strings.Split(line, ",")
	if len(parts) < 3 {
		return nil
	}

	url := parts[1]
	action := parts[2]
	return fetchRuleSet(url, action)
}

func fetchRuleSet(url string, action string) []Rule {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error fetching rule set: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var Rules []Rule
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		rule := Rule{
			Type:   parts[0],
			Text:   parts[1],
			Action: action,
		}
		// Добавляем только правила типа DOMAIN-SUFFIX и DOMAIN-KEYWORD
		if rule.Type == "DOMAIN-SUFFIX" || rule.Type == "DOMAIN-KEYWORD" {
			Rules = append(Rules, rule)
		}
	}

	return Rules
}

func MatchRules(s string, Rules []Rule) (Rule, bool) {
	rulemu.RLock()
	defer rulemu.RUnlock()
	for _, rule := range Rules {
		switch rule.Type {
		case "DOMAIN-SUFFIX":
			if strings.HasSuffix(s, rule.Text) {
				return rule, true
			}
		case "DOMAIN-KEYWORD":
			if strings.Contains(s, rule.Text) {
				return rule, true
			}
		}
	}
	return Rule{}, false
}

func PrintRules(Rules []Rule) string {
	var t string
	t += "All Rules:\n"
	for _, rule := range Rules {
		t += fmt.Sprintf("Type: %s, Text: %s, Action: %s\n",
			rule.Type, rule.Text, rule.Action)
	}
	return t
}

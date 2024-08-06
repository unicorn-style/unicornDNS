package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type Rule struct {
	Type        string
	Text        string
	Passthrough bool
	TTL         int32
	Action      string
	DNS         string
}

/*func main() {
	Rules := parseConfig("config.txt")
	printRules(Rules) // Вызов функции для вывода всех правил

	checkString := "shop.rutracker.org"
	rule, matched := matchRules(checkString, Rules)
	if matched {
		fmt.Printf("Matched Rule: %+v\n", rule)
	} else {
		fmt.Println("No matching rule found.")
	}
}*/

var Rules []Rule

func ParseConfig(filename string) []Rule {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
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
	return Rules
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
	if len(parts) < 2 {
		return nil
	}

	url := parts[1]
	return fetchRuleSet(url)
}

func fetchRuleSet(url string) []Rule {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching rule set:", err)
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
			Type: parts[0],
			Text: parts[1],
		}
		// Добавляем только правила типа DOMAIN-SUFFIX и DOMAIN-KEYWORD
		if rule.Type == "DOMAIN-SUFFIX" || rule.Type == "DOMAIN-KEYWORD" {
			Rules = append(Rules, rule)
		}
	}

	return Rules
}

func MatchRules(s string, Rules []Rule) (Rule, bool) {
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

func PrintRules(Rules []Rule) {
	fmt.Println("All Rules:")
	for _, rule := range Rules {
		fmt.Printf("Type: %s, Text: %s, Action: %s, Passthrough: %t, TTL: %d, DNS: %s\n",
			rule.Type, rule.Text, rule.Action, rule.Passthrough, rule.TTL, rule.DNS)
	}
}

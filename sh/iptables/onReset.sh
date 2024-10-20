#!/bin/bash

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 {mark} {NETTYPE}"
  exit 1
fi

MARK=$1
NETTYPE=$2

# Function delete iptables rules with mark
delete_rules_with_mark() {
  local TABLE=$1
  local CHAIN=$2
  local MARK=$3

  # Получение номеров правил с пометкой
  RULE_NUMS=$(iptables -t $TABLE -L $CHAIN --line-numbers -n | grep -E "\b$MARK\b" | awk '{print $1}' | sort -nr)

  # Удаление правил с конца списка
  for RULE_NUM in $RULE_NUMS; do
    iptables -t $TABLE -D $CHAIN $RULE_NUM
  done
}

delete_rules_with_mark6() {
  local TABLE=$1
  local CHAIN=$2
  local MARK=$3

  # Получение номеров правил с пометкой
  RULE_NUMS=$(ip6tables -t $TABLE -L $CHAIN --line-numbers -n | grep -E "\b$MARK\b" | awk '{print $1}' | sort -nr)

  # Удаление правил с конца списка
  for RULE_NUM in $RULE_NUMS; do
    ip6tables -t $TABLE -D $CHAIN $RULE_NUM
  done
}

if [ "$NETTYPE" = "ipv4" ] || [ "$NETTYPE" = "ipv4ipv6" ]; then 
delete_rules_with_mark nat PREROUTING $MARK
delete_rules_with_mark nat POSTROUTING $MARK
delete_rules_with_mark filter FORWARD $MARK
fi
if [ "$NETTYPE" = "ipv6" ] || [ "$NETTYPE" = "ipv4ipv6" ]; then 
delete_rules_with_mark6 nat PREROUTING $MARK
delete_rules_with_mark6 nat POSTROUTING $MARK
delete_rules_with_mark6 filter FORWARD $MARK
fi

echo "All rules with mark \"$MARK\" have been removed successfully"

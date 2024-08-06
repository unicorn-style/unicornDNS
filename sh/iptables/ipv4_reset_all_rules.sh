#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 {mark}"
  exit 1
fi

MARK=$1

# Функция для удаления правил с пометкой
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

# Удаление правил из таблицы nat
delete_rules_with_mark nat PREROUTING $MARK
delete_rules_with_mark nat POSTROUTING $MARK

# Удаление правил из таблицы filter
delete_rules_with_mark filter FORWARD $MARK

echo "All rules with mark \"$MARK\" have been removed successfully"

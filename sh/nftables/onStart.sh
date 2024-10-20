#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 {tablename}"
  exit 1
fi

# Название таблицы
TNAME_NAT_INPUT=$1

# Проверка наличия таблицы
if ! nft list tables | grep -q "$TNAME_NAT_INPUT"; then
    echo "nftables script ipv4_onstart.sh $TNAME_NAT_INPUT not found...Creating"
    
    # Создание таблицы и цепочек
    nft add table inet $TNAME_NAT_INPUT
    nft add chain inet $TNAME_NAT_INPUT prerouting { type nat hook prerouting priority -100 \; }
    nft add chain inet $TNAME_NAT_INPUT postrouting { type nat hook postrouting priority 100 \; }
    nft add chain inet $TNAME_NAT_INPUT forward { type filter hook forward priority 0 \; }
else
    echo "nftables script ipv4_onstart.sh $TNAME_NAT_INPUT exists"
fi
#!/bin/bash

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 {tablename} {NETTYPE}"
  exit 1
fi

TABLE_NAME_NAT=$1
NETTYPE=$2

# Удаление правил с меткой
if  [ "$NETTYPE" = "ipv4" ] || [ "$NETTYPE" = "ipv4ipv6" ]; then 
    nft flush table ip $TABLE_NAME_NAT
fi
if  [ "$NETTYPE" = "ipv6" ] || [ "$NETTYPE" = "ipv4ipv6" ]; then 
    nft flush table ip6 $TABLE_NAME_NAT
fi

echo "All rules flushed"
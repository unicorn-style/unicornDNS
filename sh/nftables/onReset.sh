#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 {tablename} {type}"
  exit 1
fi

TABLE_NAME_NAT=$1
TYPE=$2

# Удаление правил с меткой
if  ["$TYPE" = "ipv4"] || ["$TYPE" = "ipv4ipv6"]; then 
    nft flush table ip $TABLE_NAME_NAT
fi
if  ["$TYPE" = "ipv6"] || ["$TYPE" = "ipv4ipv6"]; then 
    nft flush table ip6 $TABLE_NAME_NAT
fi

echo "All rules flushed"
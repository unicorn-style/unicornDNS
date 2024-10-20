#!/bin/bash

if [ "$#" -ne 6 ]; then
  echo "Usage: $0 {inet} {localIP} {destination} {mark} {interface} {table_name_nat} {table_name_filter}"
  exit 1
fi

INET=$1
LOCAL_IP=$2
DESTINATION=$3
MARK=$4
INTERFACE=$5
TABLE_NAME_NAT=$6
# TABLE_NAME_FILTER=$6

if ["$INET" = "4"]; then
nft add rule ip $TABLE_NAME_NAT prerouting ip daddr $LOCAL_IP dnat to $DESTINATION comment \"$MARK\" &&
nft add rule ip $TABLE_NAME_NAT postrouting ip daddr $DESTINATION oif $INTERFACE masquerade comment \"$MARK\" &&
nft add rule ip $TABLE_NAME_NAT forward ip daddr $DESTINATION accept comment \"$MARK\"
fi
if ["$INET" = "6"]; then
nft add rule ip6 $TABLE_NAME_NAT prerouting ip daddr $LOCAL_IP dnat to $DESTINATION comment \"$MARK\" &&
nft add rule ip6 $TABLE_NAME_NAT postrouting ip daddr $DESTINATION oif $INTERFACE masquerade comment \"$MARK\" &&
nft add rule ip6 $TABLE_NAME_FILTER forward ip daddr $DESTINATION accept comment \"$MARK\"
fi

echo "Rules added successfully with TTL of $TTL seconds"
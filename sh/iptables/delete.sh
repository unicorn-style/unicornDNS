#!/bin/bash

if [ "$#" -ne 3 ]; then
  echo "Usage: $0 {localIP} {destination} {mark} {interface}"
  exit 1
fi

LOCAL_IP=$1
DESTINATION=$2
MARK=$3
INTERFACE=$4

# Delete rules
if [ "$INET" = "4" ]; then
iptables -t nat -D PREROUTING -d $LOCAL_IP -j DNAT --to-destination $DESTINATION -m comment --comment "$MARK" &&
iptables -t nat -D POSTROUTING -d $DESTINATION -o $INTERFACE -j MASQUERADE -m comment --comment "$MARK" &&
iptables -D FORWARD -d $DESTINATION -j ACCEPT -m comment --comment "$MARK"
fi
if [ "$INET" = "6" ]; then
ip6tables -t nat -D PREROUTING -d $LOCAL_IP -j DNAT --to-destination $DESTINATION -m comment --comment "$MARK" &&
ip6tables -t nat -D POSTROUTING -d $DESTINATION -o $INTERFACE -j MASQUERADE -m comment --comment "$MARK" &&
ip6tables -D FORWARD -d $DESTINATION -j ACCEPT -m comment --comment "$MARK"
fi

echo "Rules removed successfully"

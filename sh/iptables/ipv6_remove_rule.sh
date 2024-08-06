#!/bin/bash

if [ "$#" -ne 3 ]; then
  echo "Usage: $0 {localIP} {destination} {mark}"
  exit 1
fi

LOCAL_IP=$1
DESTINATION=$2
MARK=$3

# Удаление правил
ip6tables -t nat -D PREROUTING -d $LOCAL_IP -j DNAT --to-destination $DESTINATION -m comment --comment "$MARK"
ip6tables -t nat -D POSTROUTING -d $DESTINATION -j MASQUERADE -m comment --comment "$MARK"
ip6tables -D FORWARD -d $DESTINATION  -j ACCEPT -m comment --comment "$MARK"
ip6tables -D FORWARD -s $LOCAL_IP -o ens3 -j ACCEPT -m comment --comment "$MARK"

echo "Rules removed successfully"

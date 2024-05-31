#!/bin/bash
# exit script on error
set -e

while read -r ip; do
  ping -c 3 $ip &> /dev/null
done < <(echo "$target_ips" | tr ',' '\n')

#!/bin/sh

echo -n "Waiting for startup ."
url=$1
for i in {1..60}; do
  code=$(curl "$url" -s -o /dev/null -w "%{http_code}")
  ret=$?
  if [ $ret -eq 0 ] && [ "$code" == "200" ]; then
    echo -n " up"
    break
  fi
  echo -n .
  sleep 1
done

echo
echo "Ready @ $url"
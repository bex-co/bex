#!/bin/sh
# Runs inside the kind node (hostNetwork Traefik on :80, metrics :9100, ws meter :9101).
set -u
echo "== routed"; curl -s -H 'Host: web.onbex.test' -H 'True-Client-IP: 6.6.6.6' -H 'CF-Connecting-IP: 6.6.6.6' -H 'X-Client-IP: 6.6.6.6' -H 'Rndr-Id: forged' http://127.0.0.1/probe?key=secret | grep -iE '^(True-Client-Ip|Cf-|X-Client-Ip|Rndr-Id|X-Forwarded-For|X-Real-Ip):' | sort
echo "== blocked"; curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: blocked.onbex.test' http://127.0.0.1/denied
echo "== unknown host"; curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: nope.onbex.test' http://127.0.0.1/
echo "== websocket"; curl -s --http1.1 -o /dev/null -w '%{http_code}\n' --max-time 3 -H 'Host: web.onbex.test' -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' http://127.0.0.1/echo
sleep 1
echo "== ws meter"; curl -s http://127.0.0.1:9101/metrics | grep -v '^#' | grep -i 'srv-w8test' | sed -E 's/ [0-9.e+]+$/ <n>/' | sort
echo "== router/service metrics"; curl -s http://127.0.0.1:9100/metrics | grep -v '^#' | grep -E 'tea-w8test' | sed -E 's/\} [0-9.e+]+$/} <n>/' | grep -E 'requests_total|bytes_total' | sort

#!/bin/sh

trap "kill %1" SIGTERM

/usr/bin/env

# 24 hours of sleep is considered forever here
/bin/sleep 86400 &

wait %1

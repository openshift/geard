#!/bin/sh
for i in {1..10000}; do curl -s http://localhost:14000/scale-1.0/rest/add > /dev/null; usleep 50000; done
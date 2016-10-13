#!/usr/bin/env python
import sys, json

payload = json.loads(sys.stdin.read())

print "Payload given:"
print payload

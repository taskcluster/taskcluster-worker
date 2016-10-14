#!/usr/bin/env python
import sys, json, os

# Read the payload
payload = json.loads(sys.stdin.read())
print "Payload given:"
print payload

# Create an artifact
os.mkdir('artifacts/public/')
with open('artifacts/public/test-artifact.txt', 'w') as f:
  f.write('buildUrl given was: ' + payload.get('buildUrl'))

# Note: current working directory will be cleaned between tasks

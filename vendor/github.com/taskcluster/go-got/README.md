Got - HTTP API Calls
====================

Package got is a super simple net/http wrapper that does the right thing
for most JSON REST APIs specifically adding:

 * Retry logic with exponential back-off,
 * Reading of body with a MaxSize to avoid running out of memory,
 * Timeout after 30 seconds.

See: godoc.org/github.com/taskcluster/go-got

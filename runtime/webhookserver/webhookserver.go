package webhookserver

import "net/http"

// A WebHookServer can serve web hooks exposed to the public internet.
//
// A hook is attached with:
//         url, detach := AttachHook(handler)
// where url is the url at which the hook is exposed to the internet,
// and detach is a function that can be called to detatch the web hook.
//
// When a request to any suffix of the url is called the request is modifed to
// have resource as the suffix, and then forwarded to the given handler.
// For example, if url = "http://localhost:8080/test/", then a request to
// "http://localhost:8080/test/<suffix>" will be given to the handler as a
// request for "/<suffix>".
//
// This is useful for interactive web hooks like livelog, interactive shell and
// display.
type WebHookServer interface {
	AttachHook(handler http.Handler) (url string, detach func())
}

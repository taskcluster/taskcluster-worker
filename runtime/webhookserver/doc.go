// Package webhookserver provides implementations of the WebHookServer
// interface. Which allows attachment and detachment for web-hooks to an
// internet exposed server.
//
// The simpliest implementation is to listen on a public port and forward
// request from there. But abstracting this as an interface allows us to
// implement different approaches like to ngrok, if we ever need to run the
// worker in an environment where machines can't be exposed to the internet.
// Old windows machines in a data center seems like a plausible use-case.
package webhookserver

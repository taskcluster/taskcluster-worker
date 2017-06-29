// Package webhookserver provides implementations of the WebHookServer
// interface. Which allows attachment and detachment for web-hooks to an
// internet exposed server.
//
// The simplest implementation is to listen on a public port and forward
// request from there. But abstracting this as an interface allows us to
// implement different approaches like to ngrok, if we ever need to run the
// worker in an environment where machines can't be exposed to the internet.
// Old windows machines in a data center seems like a plausible use-case.
//
// Webhooktunnel is an implementation of webhookserver which allows exposing
// endpoints without having to open any ports, public or otherwise. It uses
// webhookclient to connect to the proxy and multiplexes streams over websocket,
// and exposes a net.Listener interface. This allows http to be served using
// the client object as a listener. This results in a more secure worker.
// Webhooktunnel requires TC credentials.
package webhookserver

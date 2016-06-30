// Package got is a super simple net/http wrapper that does the right thing
// for most JSON REST APIs specifically adding:
//  - Retry logic with exponential back-off,
//  - Reading of body with a MaxSize to avoid running out of memory,
//  - Timeout after 30 seconds.
//
// Send a request with retries like this:
//
//        got := got.New()
//        response, err := got.NewRequest("PUT", url, []byte("...")).Send()
//        if err == nil {
//          // handle error
//        }
//        json.Unmarshal(response.Body, &MyObject)
//
// This package will never support streaming requests. For small requests
// like JSON it is often best to buffer the entire body before parsing it.
// As slow parsing using a unbuffered decoder can slowdown the request.
//
// It is also much easier to send requests and handled responses with small
// JSON payloads if there is no dealing with streaming. Streaming particularly
// complicates retries. If you need to stream the request body, DO NOT use
// this package.
package got

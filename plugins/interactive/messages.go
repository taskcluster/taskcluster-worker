package interactive

// Message type codes for websocket messages implementing interactive shell.
//
// We will send stdin, stdout, stderr, exit and abort messages over a
// websocket. Messages will all have the form: [type] [data]
//
// Where [type] is a single byte with the value of MessageTypeData
// MessageTypeAbort or MessageTypeExit.
// The data property depends on the [type] of the message, as outlined below.
//
// If [type] is MessageTypeData then
//   [data] = [stream] [payload]
// , where  [stream] is a single byte: StreamStdin, StreamStdout, StreamStderr,
// and [payload] is data from this stream. If [payload] is an empty byte
// sequence this signals the end of the stream.
//
// If [type] is MessageTypeAck then
//   [data] = [stream] [N]
// , where [stream] is a single byte: StreamStdin, StreamStdout, StreamStderr,
// and [N] is a big-endian 32 bit unsigned integer acknowleging the remote
// stream to have processed N bytes.
//
// If [type] is MessageTypeAbort then [data] is empty, this message is used
// to request that the executing be aborted.
//
// If [type] is MessageTypeExit then [data] = [exitCode], where exitCode is a
// single byte 0 (success) or 1 (failed) indicating whether the command
// terminated successfully.
//
// If [type] is MessageTypeSize then
//   [data] = [colmns] [rows]
// , where [colmns] and [rows] are big-endian 16 bit unsigned integers
// specifying the width and height of the TTY. If not supported this message is
// is ignored.
const (
	MessageTypeData  = 0
	MessageTypeAck   = 1
	MessageTypeSize  = 3
	MessageTypeAbort = 4
	MessageTypeExit  = 5
	StreamStdin      = 0
	StreamStdout     = 1
	StreamStderr     = 2
)

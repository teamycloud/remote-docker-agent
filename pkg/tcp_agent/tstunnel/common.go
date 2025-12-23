package tstunnel

import "io"

// agentDialResult provides asynchronous agent dialing results.
// This type is shared across forwarding and synchronization protocols.
type agentDialResult struct {
	// stream is the stream returned by agent dialing.
	stream io.ReadWriteCloser
	// error is the error returned by agent dialing.
	error error
}

// Package ipc provides inter-process communication functionality.
//
// This package implements IPC using platform-specific mechanisms:
//   - Windows: Named Pipes
//   - Linux: Unix Domain Sockets
//
// The package provides both client and server implementations with
// built-in encryption using ECDH key exchange and AES-GCM encryption.
package ipc

import "fmt"

// IpcServer represents an IPC server that can accept multiple client connections.
//
// IpcServer uses the PIMPL (Pointer to Implementation) pattern to hide
// platform-specific implementation details.
//
// Example:
//
//	server := NewIpcServer()
//	err := server.Start("my-server", func(id ClientId, data []byte, sendCb ServerSendDataCallback) uint32 {
//	    // Handle incoming data
//	    return uint32(len(data))
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Stop()
type IpcServer struct {
	impl *serverImpl
}

// NewIpcServer creates a new IPC server instance.
//
// Returns:
//   - *IpcServer: A new IPC server instance
func NewIpcServer() *IpcServer {
	return &IpcServer{
		impl: newServerImpl(),
	}
}

// Start begins listening for client connections.
//
// Parameters:
//   - serverName: The name of the server endpoint
//   - callback: Function called when data is received from a client
//
// Returns:
//   - error: nil on success, or an error on failure
//
// The callback function receives:
//   - id: The client identifier
//   - data: The received data (decrypted)
//   - sendCb: Function to send data back to the client
//
// The callback should return the number of bytes consumed from the data buffer.
func (s *IpcServer) Start(serverName string, callback ServerRecvDataCallback) error {
	return s.impl.Start(serverName, callback)
}

// Stop shuts down the server and disconnects all clients.
//
// This method is idempotent and can be called multiple times safely.
func (s *IpcServer) Stop() {
	s.impl.Stop()
}

// SendToClient sends data to a specific client.
//
// Parameters:
//   - id: The client identifier
//   - data: The data to send (will be encrypted)
//
// Returns:
//   - error: nil on success, or an error on failure
func (s *IpcServer) SendToClient(id ClientId, data []byte) error {
	return s.impl.SendToClient(id, data)
}

// IpcConnection represents a client connection to an IPC server.
//
// IpcConnection handles the encryption handshake automatically during Connect().
//
// Example:
//
//	conn := NewIpcConnection()
//	err := conn.Connect("my-server", 5000, 5000)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
type IpcConnection struct {
	impl *clientConnectionImpl
}

// NewIpcConnection creates a new IPC connection instance.
//
// Returns:
//   - *IpcConnection: A new IPC connection instance
func NewIpcConnection() *IpcConnection {
	return &IpcConnection{
		impl: newClientConnectionImpl(),
	}
}

// Connect establishes a connection to an IPC server.
//
// Parameters:
//   - serverName: The name of the server endpoint
//   - timeoutMs: Connection timeout in milliseconds
//   - rwTimeoutMs: Read/write timeout in milliseconds
//
// Returns:
//   - error: nil on success, or an error on failure
//
// Note: This method performs an encryption handshake automatically.
func (c *IpcConnection) Connect(serverName string, timeoutMs uint32, rwTimeoutMs uint32) error {
	return c.impl.Connect(serverName, timeoutMs, rwTimeoutMs)
}

// WriteEncrypted sends encrypted data to the server.
//
// Parameters:
//   - data: The data to send (will be encrypted)
//   - timeoutMs: Write timeout in milliseconds
//
// Returns:
//   - error: nil on success, or an error on failure
func (c *IpcConnection) WriteEncrypted(data []byte, timeoutMs uint32) error {
	return c.impl.WriteEncrypted(data, timeoutMs)
}

// ReadLoop starts a blocking read loop that receives and decrypts data.
//
// Parameters:
//   - callback: Function called when data is received
//   - timeoutMs: Read timeout in milliseconds
//
// Returns:
//   - error: nil on success, or an error on failure
//
// The callback should return true to stop the read loop, or false to continue.
func (c *IpcConnection) ReadLoop(callback ClientRecvDataCallback, timeoutMs uint32) error {
	return c.impl.ReadLoop(callback, timeoutMs)
}

// Close closes the connection and releases resources.
func (c *IpcConnection) Close() {
	c.impl.Close()
}

// IsValid checks if the connection is valid and ready for use.
//
// Returns:
//   - bool: true if the connection is valid, false otherwise
func (c *IpcConnection) IsValid() bool {
	return c.impl.IsValid()
}

// IpcClientSend sends a request to an IPC server and waits for a response.
//
// This is a convenience function that handles connection lifecycle automatically.
//
// Parameters:
//   - serverName: The name of the server endpoint
//   - request: The request data to send
//   - callback: Function called when response is received
//   - timeoutMs: Timeout in milliseconds
//
// Returns:
//   - error: nil on success, or an error on failure
func IpcClientSend(serverName string, request []byte, callback ClientRecvDataCallback, timeoutMs uint32) error {
	conn := NewIpcConnection()
	if err := conn.Connect(serverName, timeoutMs, timeoutMs); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close()

	if err := conn.WriteEncrypted(request, timeoutMs); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return conn.ReadLoop(callback, timeoutMs)
}

// IpcClientSendAsync sends a request to an IPC server without waiting for a response.
//
// This is a fire-and-forget function that returns immediately.
//
// Parameters:
//   - serverName: The name of the server endpoint
//   - request: The request data to send
//   - timeoutMs: Timeout in milliseconds
//
// Returns:
//   - error: nil (always returns nil, errors are silently ignored)
func IpcClientSendAsync(serverName string, request []byte, timeoutMs uint32) error {
	go func() {
		conn := NewIpcConnection()
		defer conn.Close()
		if err := conn.Connect(serverName, timeoutMs, timeoutMs); err != nil {
			return
		}
		conn.WriteEncrypted(request, timeoutMs)
	}()
	return nil
}

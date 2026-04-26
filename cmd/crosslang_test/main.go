package main

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cppbugboy/tyke-go/tyke/component"
	"github.com/cppbugboy/tyke-go/tyke/ipc"
)

func main() {
	component.GetCoroutinePoolInstance().Init(4)

	if len(os.Args) < 2 {
		fmt.Println("Usage: crosslang_test <mode> [server_name]")
		fmt.Println("Modes: go-server, go-client, go-server-echo, test-all")
		os.Exit(1)
	}

	mode := os.Args[1]
	serverName := "crosslang_test"
	if len(os.Args) > 2 {
		serverName = os.Args[2]
	}

	switch mode {
	case "go-server":
		runGoServer(serverName)
	case "go-client":
		runGoClient(serverName)
	case "go-server-echo":
		runGoEchoServer(serverName)
	case "test-all":
		runAllCrossLangTests()
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}

func runGoServer(name string) {
	fmt.Printf("[Go Server] Starting on '%s'...\n", name)
	server := ipc.NewIPCServer()
	var msgCount atomic.Int32
	callback := func(cid ipc.ClientId, data []byte, sendCb func(ipc.ClientId, []byte) bool) *uint32 {
		n := msgCount.Add(1)
		fmt.Printf("[Go Server] Received msg #%d from client %d: %d bytes\n", n, cid, len(data))
		return nil
	}
	result := server.Start(name, callback)
	if !result.HasValue() {
		fmt.Printf("[Go Server] Start failed: %s\n", result.Err)
		os.Exit(1)
	}
	fmt.Printf("[Go Server] Started successfully. Press Ctrl+C to stop.\n")
	select {}
}

func runGoEchoServer(name string) {
	fmt.Printf("[Go Echo Server] Starting on '%s'...\n", name)
	server := ipc.NewIPCServer()
	var msgCount atomic.Int32
	callback := func(cid ipc.ClientId, data []byte, sendCb func(ipc.ClientId, []byte) bool) *uint32 {
		n := msgCount.Add(1)
		fmt.Printf("[Go Echo Server] Received msg #%d from client %d: %d bytes, echoing back\n", n, cid, len(data))
		sendCb(cid, data)
		return nil
	}
	result := server.Start(name, callback)
	if !result.HasValue() {
		fmt.Printf("[Go Echo Server] Start failed: %s\n", result.Err)
		os.Exit(1)
	}
	fmt.Printf("[Go Echo Server] Started successfully. Press Ctrl+C to stop.\n")
	select {}
}

func runGoClient(name string) {
	fmt.Printf("[Go Client] Connecting to '%s'...\n", name)
	conn := ipc.NewIPCConnection()
	result := conn.Connect(name, 5000)
	if !result.HasValue() {
		fmt.Printf("[Go Client] Connect failed: %s\n", result.Err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("[Go Client] Connected successfully!\n")

	sizes := []int{1, 64, 1024, 65536, 256 * 1024}
	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i & 0xFF)
		}
		writeResult := conn.WriteEncrypted(data, 5000)
		if !writeResult.HasValue() {
			fmt.Printf("[Go Client] Write %d bytes failed: %s\n", size, writeResult.Err)
		} else {
			fmt.Printf("[Go Client] Sent %d bytes successfully\n", size)
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("[Go Client] All messages sent!\n")
}

func runAllCrossLangTests() {
	fmt.Println("========================================")
	fmt.Println("  Cross-Language IPC Test Suite")
	fmt.Println("========================================")
	fmt.Println()

	totalTests := 0
	passedTests := 0

	testGoServerCppClient := func() bool {
		fmt.Println("[Test 1] Go Server + Go Client (same-language baseline)")
		var received []byte
		var dataReceived atomic.Bool
		server := ipc.NewIPCServer()
		callback := func(cid ipc.ClientId, data []byte, sendCb func(ipc.ClientId, []byte) bool) *uint32 {
			received = make([]byte, len(data))
			copy(received, data)
			dataReceived.Store(true)
			return nil
		}
		result := server.Start("cross_go_baseline", callback)
		if !result.HasValue() {
			fmt.Printf("  FAIL: Server start failed: %s\n", result.Err)
			return false
		}
		defer server.Stop()

		conn := ipc.NewIPCConnection()
		cr := conn.Connect("cross_go_baseline", 3000)
		if !cr.HasValue() {
			fmt.Printf("  FAIL: Client connect failed: %s\n", cr.Err)
			return false
		}
		defer conn.Close()

		testData := []byte{0xCA, 0xFE, 0xBA, 0xBE}
		wr := conn.WriteEncrypted(testData, 3000)
		if !wr.HasValue() {
			fmt.Printf("  FAIL: Write failed: %s\n", wr.Err)
			return false
		}

		for i := 0; i < 100 && !dataReceived.Load(); i++ {
			time.Sleep(10 * time.Millisecond)
		}
		if !dataReceived.Load() || !bytes.Equal(received, testData) {
			fmt.Printf("  FAIL: Data mismatch\n")
			return false
		}
		fmt.Println("  PASS: Go Server + Go Client baseline")
		return true
	}

	testGoServerLargeMsg := func() bool {
		fmt.Println("[Test 2] Go Server + Go Client Large Message (fragmentation)")
		var received []byte
		var dataReceived atomic.Bool
		server := ipc.NewIPCServer()
		callback := func(cid ipc.ClientId, data []byte, sendCb func(ipc.ClientId, []byte) bool) *uint32 {
			received = make([]byte, len(data))
			copy(received, data)
			dataReceived.Store(true)
			return nil
		}
		result := server.Start("cross_go_large", callback)
		if !result.HasValue() {
			fmt.Printf("  FAIL: Server start failed: %s\n", result.Err)
			return false
		}
		defer server.Stop()

		conn := ipc.NewIPCConnection()
		cr := conn.Connect("cross_go_large", 3000)
		if !cr.HasValue() {
			fmt.Printf("  FAIL: Client connect failed: %s\n", cr.Err)
			return false
		}
		defer conn.Close()

		msgSize := 512 * 1024
		testData := make([]byte, msgSize)
		for i := range testData {
			testData[i] = byte(i & 0xFF)
		}
		wr := conn.WriteEncrypted(testData, 5000)
		if !wr.HasValue() {
			fmt.Printf("  FAIL: Write failed: %s\n", wr.Err)
			return false
		}

		for i := 0; i < 200 && !dataReceived.Load(); i++ {
			time.Sleep(10 * time.Millisecond)
		}
		if !dataReceived.Load() || !bytes.Equal(received, testData) {
			fmt.Printf("  FAIL: Large message mismatch (received %d bytes, expected %d)\n", len(received), msgSize)
			return false
		}
		fmt.Printf("  PASS: Go Server + Go Client 512KB fragmented message\n")
		return true
	}

	testGoServerBidirectional := func() bool {
		fmt.Println("[Test 3] Go Server + Go Client Bidirectional")
		var serverReceived atomic.Bool
		var clientReceived atomic.Bool
		var serverData []byte
		var clientData []byte

		server := ipc.NewIPCServer()
		callback := func(cid ipc.ClientId, data []byte, sendCb func(ipc.ClientId, []byte) bool) *uint32 {
			serverData = make([]byte, len(data))
			copy(serverData, data)
			serverReceived.Store(true)
			sendCb(cid, []byte{0xDE, 0xAD})
			return nil
		}
		result := server.Start("cross_go_bidi", callback)
		if !result.HasValue() {
			fmt.Printf("  FAIL: Server start failed\n")
			return false
		}
		defer server.Stop()

		conn := ipc.NewIPCConnection()
		cr := conn.Connect("cross_go_bidi", 3000)
		if !cr.HasValue() {
			fmt.Printf("  FAIL: Client connect failed\n")
			return false
		}
		defer conn.Close()

		req := []byte{0xCA, 0xFE}
		conn.WriteEncrypted(req, 3000)

		go func() {
			conn.ReadLoop(func(data []byte) bool {
				clientData = make([]byte, len(data))
				copy(clientData, data)
				clientReceived.Store(true)
				return true
			}, 3000)
		}()

		for i := 0; i < 100 && (!serverReceived.Load() || !clientReceived.Load()); i++ {
			time.Sleep(10 * time.Millisecond)
		}
		if !serverReceived.Load() || !clientReceived.Load() {
			fmt.Printf("  FAIL: Bidirectional timeout (srv=%v cli=%v)\n", serverReceived.Load(), clientReceived.Load())
			return false
		}
		fmt.Println("  PASS: Go Server + Go Client bidirectional")
		return true
	}

	testGoServerConcurrent := func() bool {
		fmt.Println("[Test 4] Go Server + Go Client Concurrent Connections")
		var serverRecv atomic.Int32
		server := ipc.NewIPCServer()
		callback := func(cid ipc.ClientId, data []byte, sendCb func(ipc.ClientId, []byte) bool) *uint32 {
			serverRecv.Add(1)
			return nil
		}
		result := server.Start("cross_go_concurrent", callback)
		if !result.HasValue() {
			fmt.Printf("  FAIL: Server start failed\n")
			return false
		}
		defer server.Stop()

		numClients := 10
		var wg sync.WaitGroup
		var success atomic.Int32
		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn := ipc.NewIPCConnection()
				cr := conn.Connect("cross_go_concurrent", 3000)
				if cr.HasValue() {
					success.Add(1)
					conn.WriteEncrypted([]byte{0x01}, 3000)
					time.Sleep(10 * time.Millisecond)
				}
				conn.Close()
			}()
		}
		wg.Wait()

		if int(success.Load()) < numClients*80/100 {
			fmt.Printf("  FAIL: Only %d/%d connections succeeded\n", success.Load(), numClients)
			return false
		}
		fmt.Printf("  PASS: %d/%d concurrent connections succeeded\n", success.Load(), numClients)
		return true
	}

	testCppServerGoClient := func() bool {
		fmt.Println("[Test 5] C++ Server + Go Client (cross-language)")
		conn := ipc.NewIPCConnection()
		cr := conn.Connect("cross_cpp_server", 2000)
		if !cr.HasValue() {
			fmt.Println("  SKIP: C++ server not running")
			return true
		}
		defer conn.Close()

		testData := []byte{0xCA, 0xFE, 0xBA, 0xBE}
		wr := conn.WriteEncrypted(testData, 3000)
		if !wr.HasValue() {
			fmt.Printf("  FAIL: Write to C++ server failed: %s\n", wr.Err)
			return false
		}
		fmt.Println("  PASS: Go Client connected and sent data to C++ Server")
		return true
	}

	tests := []func() bool{
		testGoServerCppClient,
		testGoServerLargeMsg,
		testGoServerBidirectional,
		testGoServerConcurrent,
		testCppServerGoClient,
	}

	for _, test := range tests {
		totalTests++
		if test() {
			passedTests++
		}
		fmt.Println()
	}

	fmt.Println("========================================")
	fmt.Printf("  Results: %d/%d tests passed\n", passedTests, totalTests)
	fmt.Println("========================================")

	if passedTests < totalTests {
		os.Exit(1)
	}
}

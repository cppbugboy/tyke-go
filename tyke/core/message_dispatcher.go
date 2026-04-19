package core

import (
	"encoding/binary"
	"sync"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/ipc"
)

type MessageHandler func(clientId ipc.ClientId, data []byte, used *uint32)

var (
	messageHandlers   map[common.MessageType]MessageHandler
	messageHandlersMu sync.Mutex
)

func init() {
	messageHandlers = make(map[common.MessageType]MessageHandler)
}

func RegisterMessageHandler(msgType common.MessageType, handler MessageHandler) {
	messageHandlersMu.Lock()
	defer messageHandlersMu.Unlock()
	messageHandlers[msgType] = handler
}

func DispatchMessage(clientId ipc.ClientId, data []byte, used *uint32) {
	var header common.ProtocolHeader
	copy(header.Magic[:], data[:4])
	header.MsgType = common.MessageType(binary.LittleEndian.Uint32(data[4:8]))

	messageHandlersMu.Lock()
	handler, ok := messageHandlers[header.MsgType]
	messageHandlersMu.Unlock()

	if ok {
		handler(clientId, data, used)
	}
}

func HasMessageHandler(msgType common.MessageType) bool {
	messageHandlersMu.Lock()
	defer messageHandlersMu.Unlock()
	_, ok := messageHandlers[msgType]
	return ok
}

// Package core implements the Tyke framework kernel.
//
// This file provides wire-format encoding and decoding for Request and Response
// objects using the Tyke protocol header + JSON metadata + raw content layout.
package core

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"tyke-go/common"
)

// EncodeRequest serializes a Request into the Tyke wire format.
func EncodeRequest(request *Request) ([]byte, error) {
	common.LogInfo("Encoding request", "route", request.GetRoute())
	return encodeRequest(request)
}

// DecodeRequest deserializes a byte slice into a Request. On success, dataSize is set
// to the number of bytes consumed.
func DecodeRequest(dataVec []byte, request *Request, dataSize *uint32) bool {
	common.LogInfo("Decoding request", "size", len(dataVec))
	return decode(dataVec, request, dataSize)
}

// EncodeResponse serializes a Response into the Tyke wire format.
func EncodeResponse(response *Response) ([]byte, error) {
	common.LogInfo("Encoding response", "route", response.GetRoute())
	return encodeResponse(response)
}

// DecodeResponse deserializes a byte slice into a Response. On success, dataSize is set
// to the number of bytes consumed.
func DecodeResponse(dataVec []byte, response *Response, dataSize *uint32) bool {
	common.LogInfo("Decoding response", "size", len(dataVec))
	return decode(dataVec, response, dataSize)
}

// encodeRequest marshals request metadata to JSON and delegates to encodeCommon.
func encodeRequest(request *Request) ([]byte, error) {
	metadataBytes, err := json.Marshal(&request.metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata serialization failed: %w", err)
	}
	return encodeCommon(&request.protocolHeader, string(metadataBytes), request.content)
}

// encodeResponse marshals response metadata to JSON and delegates to encodeCommon.
func encodeResponse(response *Response) ([]byte, error) {
	metadataBytes, err := json.Marshal(&response.metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata serialization failed: %w", err)
	}
	return encodeCommon(&response.protocolHeader, string(metadataBytes), response.content)
}

// encodeCommon builds a complete Tyke protocol frame: [header][metadata bytes][content bytes].
// All integer fields are little-endian.
func encodeCommon(ph *common.ProtocolHeader, metadataString string, content []byte) ([]byte, error) {
	metaBytes := []byte(metadataString)
	contentBytes := content

	headerSize := uint32(common.ProtocolHeaderSize)
	metaSize := uint32(len(metaBytes))
	contentSize := uint32(len(contentBytes))
	totalSize := headerSize + metaSize + contentSize

	ph.MetadataLen = metaSize
	ph.ContentLen = contentSize

	dataVec := make([]byte, totalSize)

	offset := uint32(0)
	copy(dataVec[offset:offset+4], ph.Magic[:])
	offset += 4
	binary.LittleEndian.PutUint32(dataVec[offset:], uint32(ph.MsgType))
	offset += 4
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(dataVec[offset:], ph.Reserved[i])
		offset += 4
	}
	binary.LittleEndian.PutUint32(dataVec[offset:], ph.MetadataLen)
	offset += 4
	binary.LittleEndian.PutUint32(dataVec[offset:], ph.ContentLen)
	offset += 4

	if metaSize > 0 {
		copy(dataVec[headerSize:], metaBytes)
	}
	if contentSize > 0 {
		copy(dataVec[headerSize+metaSize:], contentBytes)
	}

	common.LogDebug("Encode completed", "header", headerSize, "metadata", metaSize, "content", contentSize, "total", totalSize)
	return dataVec, nil
}

type decodable interface {
	setProtocolHeader(common.ProtocolHeader)
	setMetadataFromJson(string) error
	setContent([]byte)
}

func (r *Request) setProtocolHeader(ph common.ProtocolHeader) { r.protocolHeader = ph }
func (r *Request) setMetadataFromJson(s string) error         { return r.metadata.FromJsonString(s) }
func (r *Request) setContent(c []byte)                        { r.content = c }

func (r *Response) setProtocolHeader(ph common.ProtocolHeader) { r.protocolHeader = ph }
func (r *Response) setMetadataFromJson(s string) error         { return r.metadata.FromJsonString(s) }
func (r *Response) setContent(c []byte)                        { r.content = c }

// decode parses a byte slice into a decodable message (Request or Response).
// It validates magic, length fields, and handles integer overflow checks.
func decode(dataVec []byte, msg decodable, dataSize *uint32) bool {
	*dataSize = 0
	vecSize := uint32(len(dataVec))
	headerSize := uint32(common.ProtocolHeaderSize)

	if vecSize < headerSize {
		common.LogError("Data too short for header", "expected", headerSize, "got", vecSize)
		return false
	}

	var ph common.ProtocolHeader
	offset := uint32(0)
	copy(ph.Magic[:], dataVec[offset:offset+4])
	offset += 4
	ph.MsgType = common.MessageType(binary.LittleEndian.Uint32(dataVec[offset:]))
	offset += 4
	for i := 0; i < 3; i++ {
		ph.Reserved[i] = binary.LittleEndian.Uint32(dataVec[offset:])
		offset += 4
	}
	ph.MetadataLen = binary.LittleEndian.Uint32(dataVec[offset:])
	offset += 4
	ph.ContentLen = binary.LittleEndian.Uint32(dataVec[offset:])
	offset += 4

	if ph.Magic != common.ProtocolMagic {
		common.LogError("Protocol magic mismatch")
		return false
	}

	metaLen := ph.MetadataLen
	contLen := ph.ContentLen

	const maxMetadataLen uint32 = 256 * 1024     // 256KB
	const maxContentLen uint32 = 1 * 1024 * 1024 // 1MB

	if metaLen > maxMetadataLen {
		common.LogError("Metadata length exceeds limit", "len", metaLen, "max", maxMetadataLen)
		return false
	}

	if contLen > maxContentLen {
		common.LogError("Content length exceeds limit", "len", contLen, "max", maxContentLen)
		return false
	}

	// Guard against integer overflow when adding metaLen and contLen.
	if uint64(metaLen)+uint64(contLen) > uint64(^uint32(0)) {
		common.LogError("Metadata + content length overflow", "meta", metaLen, "content", contLen)
		return false
	}

	if vecSize < headerSize+metaLen+contLen {
		common.LogError("Data incomplete", "expected", headerSize+metaLen+contLen, "got", vecSize)
		return false
	}

	if metaLen > 0 {
		metaStr := string(dataVec[headerSize : headerSize+metaLen])
		if err := msg.setMetadataFromJson(metaStr); err != nil {
			common.LogError("Metadata deserialization failed", "error", err)
			return false
		}
	}

	if contLen > 0 {
		contentStart := headerSize + metaLen
		content := make([]byte, contLen)
		copy(content, dataVec[contentStart:contentStart+contLen])
		msg.setContent(content)
	}

	*dataSize = headerSize + metaLen + contLen
	msg.setProtocolHeader(ph)

	common.LogDebug("Decode completed", "header", headerSize, "metadata", metaLen, "content", contLen, "total", *dataSize)
	return true
}

package core

import (
	"encoding/binary"
	"fmt"

	"github.com/tyke/tyke/pkg/common"
)

type DataProc struct{}

func EncodeRequest(req *TykeRequest) ([]byte, error) {
	metadataJson, err := req.metadata.ToJsonString()
	if err != nil {
		return nil, fmt.Errorf("encode request: metadata marshal failed: %w", err)
	}

	metadataBytes := []byte(metadataJson)
	contentBytes := req.content

	header := common.ProtocolHeader{
		Magic:       common.ProtocolMagic,
		MsgType:     req.msgType,
		Reserved:    [3]uint32{0, 0, 0},
		MetadataLen: uint32(len(metadataBytes)),
		ContentLen:  uint32(len(contentBytes)),
	}

	headerBytes := serializeProtocolHeader(&header)

	totalSize := len(headerBytes) + len(metadataBytes) + len(contentBytes)
	result := make([]byte, 0, totalSize)
	result = append(result, headerBytes...)
	result = append(result, metadataBytes...)
	result = append(result, contentBytes...)

	return result, nil
}

func DecodeRequest(data []byte) (*TykeRequest, uint32, error) {
	if len(data) < common.ProtocolHeaderSize {
		return nil, 0, fmt.Errorf("decode request: data too small (%d < %d)", len(data), common.ProtocolHeaderSize)
	}

	header, err := deserializeProtocolHeader(data[:common.ProtocolHeaderSize])
	if err != nil {
		return nil, 0, fmt.Errorf("decode request: header deserialization failed: %w", err)
	}

	if header.Magic != common.ProtocolMagic {
		return nil, 0, fmt.Errorf("decode request: invalid magic %v", header.Magic)
	}

	totalDataSize := common.ProtocolHeaderSize + header.MetadataLen + header.ContentLen
	if uint32(len(data)) < totalDataSize {
		return nil, 0, fmt.Errorf("decode request: incomplete data (need %d, have %d)", totalDataSize, len(data))
	}

	metadataStart := common.ProtocolHeaderSize
	metadataEnd := metadataStart + int(header.MetadataLen)
	contentStart := metadataEnd

	metadataJson := string(data[metadataStart:metadataEnd])

	req := AcquireRequest()
	req.msgType = header.MsgType
	if err := req.metadata.FromJsonString(metadataJson); err != nil {
		ReleaseRequest(req)
		return nil, 0, fmt.Errorf("decode request: metadata unmarshal failed: %w", err)
	}

	if header.ContentLen > 0 {
		req.content = make([]byte, header.ContentLen)
		copy(req.content, data[contentStart:contentStart+int(header.ContentLen)])
	}

	return req, totalDataSize, nil
}

func EncodeResponse(resp *TykeResponse) ([]byte, error) {
	metadataJson, err := resp.metadata.ToJsonString()
	if err != nil {
		return nil, fmt.Errorf("encode response: metadata marshal failed: %w", err)
	}

	metadataBytes := []byte(metadataJson)
	contentBytes := resp.content

	header := common.ProtocolHeader{
		Magic:       common.ProtocolMagic,
		MsgType:     resp.msgType,
		Reserved:    [3]uint32{0, 0, 0},
		MetadataLen: uint32(len(metadataBytes)),
		ContentLen:  uint32(len(contentBytes)),
	}

	headerBytes := serializeProtocolHeader(&header)

	totalSize := len(headerBytes) + len(metadataBytes) + len(contentBytes)
	result := make([]byte, 0, totalSize)
	result = append(result, headerBytes...)
	result = append(result, metadataBytes...)
	result = append(result, contentBytes...)

	return result, nil
}

func DecodeResponse(data []byte) (*TykeResponse, uint32, error) {
	if len(data) < common.ProtocolHeaderSize {
		return nil, 0, fmt.Errorf("decode response: data too small (%d < %d)", len(data), common.ProtocolHeaderSize)
	}

	header, err := deserializeProtocolHeader(data[:common.ProtocolHeaderSize])
	if err != nil {
		return nil, 0, fmt.Errorf("decode response: header deserialization failed: %w", err)
	}

	if header.Magic != common.ProtocolMagic {
		return nil, 0, fmt.Errorf("decode response: invalid magic %v", header.Magic)
	}

	totalDataSize := common.ProtocolHeaderSize + header.MetadataLen + header.ContentLen
	if uint32(len(data)) < totalDataSize {
		return nil, 0, fmt.Errorf("decode response: incomplete data (need %d, have %d)", totalDataSize, len(data))
	}

	metadataStart := common.ProtocolHeaderSize
	metadataEnd := metadataStart + int(header.MetadataLen)
	contentStart := metadataEnd

	metadataJson := string(data[metadataStart:metadataEnd])

	resp := AcquireResponse()
	resp.msgType = header.MsgType
	if err := resp.metadata.FromJsonString(metadataJson); err != nil {
		ReleaseResponse(resp)
		return nil, 0, fmt.Errorf("decode response: metadata unmarshal failed: %w", err)
	}

	if header.ContentLen > 0 {
		resp.content = make([]byte, header.ContentLen)
		copy(resp.content, data[contentStart:contentStart+int(header.ContentLen)])
	}

	return resp, totalDataSize, nil
}

func serializeProtocolHeader(header *common.ProtocolHeader) []byte {
	buf := make([]byte, common.ProtocolHeaderSize)
	off := 0
	copy(buf[off:off+4], header.Magic[:])
	off += 4
	binary.LittleEndian.PutUint32(buf[off:off+4], uint32(header.MsgType))
	off += 4
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(buf[off:off+4], header.Reserved[i])
		off += 4
	}
	binary.LittleEndian.PutUint32(buf[off:off+4], header.MetadataLen)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:off+4], header.ContentLen)
	return buf
}

func deserializeProtocolHeader(data []byte) (*common.ProtocolHeader, error) {
	if len(data) < common.ProtocolHeaderSize {
		return nil, fmt.Errorf("data too small for protocol header")
	}
	header := &common.ProtocolHeader{}
	off := 0
	copy(header.Magic[:], data[off:off+4])
	off += 4
	header.MsgType = common.MessageType(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4
	for i := 0; i < 3; i++ {
		header.Reserved[i] = binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
	}
	header.MetadataLen = binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	header.ContentLen = binary.LittleEndian.Uint32(data[off : off+4])
	return header, nil
}

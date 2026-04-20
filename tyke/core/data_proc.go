package core

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/tyke/tyke/tyke/common"
)

func EncodeRequest(request *TykeRequest) ([]byte, error) {
	return encodeRequest(request)
}

func DecodeRequest(dataVec []byte, request *TykeRequest, dataSize *uint32) bool {
	return decode(dataVec, request, dataSize)
}

func EncodeResponse(response *TykeResponse) ([]byte, error) {
	return encodeResponse(response)
}

func DecodeResponse(dataVec []byte, response *TykeResponse, dataSize *uint32) bool {
	return decode(dataVec, response, dataSize)
}

func encodeRequest(request *TykeRequest) ([]byte, error) {
	metadataBytes, err := json.Marshal(&request.metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata serialization failed: %w", err)
	}
	return encodeCommon(&request.protocolHeader, string(metadataBytes), request.content)
}

func encodeResponse(response *TykeResponse) ([]byte, error) {
	metadataBytes, err := json.Marshal(&response.metadata)
	if err != nil {
		return nil, fmt.Errorf("metadata serialization failed: %w", err)
	}
	return encodeCommon(&response.protocolHeader, string(metadataBytes), response.content)
}

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

func (r *TykeRequest) setProtocolHeader(ph common.ProtocolHeader) { r.protocolHeader = ph }
func (r *TykeRequest) setMetadataFromJson(s string) error         { return r.metadata.FromJsonString(s) }
func (r *TykeRequest) setContent(c []byte)                        { r.content = c }

func (r *TykeResponse) setProtocolHeader(ph common.ProtocolHeader) { r.protocolHeader = ph }
func (r *TykeResponse) setMetadataFromJson(s string) error         { return r.metadata.FromJsonString(s) }
func (r *TykeResponse) setContent(c []byte)                        { r.content = c }

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

	const maxMetadataLen uint32 = 4 * 1024 * 1024
	const maxContentLen uint32 = 64 * 1024 * 1024

	if metaLen > maxMetadataLen {
		common.LogError("Metadata length exceeds limit", "len", metaLen, "max", maxMetadataLen)
		return false
	}

	if contLen > maxContentLen {
		common.LogError("Content length exceeds limit", "len", contLen, "max", maxContentLen)
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

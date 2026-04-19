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
	metadataBytes, err := json.Marshal(&struct {
		Module      string `json:"module"`
		AsyncUuid   string `json:"async_uuid"`
		MsgUuid     string `json:"msg_uuid"`
		Route       string `json:"route"`
		ContentType string `json:"content_type"`
		Timestamp   string `json:"timestamp"`
	}{
		Module:      request.metadata.Module,
		AsyncUuid:   request.metadata.AsyncUuid,
		MsgUuid:     request.metadata.MsgUuid,
		Route:       request.metadata.Route,
		ContentType: request.metadata.ContentType,
		Timestamp:   request.metadata.Timestamp,
	})
	if err != nil {
		return nil, fmt.Errorf("metadata serialization failed: %w", err)
	}
	metadataString := string(metadataBytes)

	return encodeCommon(&request.protocolHeader, metadataString, request.content)
}

func encodeResponse(response *TykeResponse) ([]byte, error) {
	metadataBytes, err := json.Marshal(&struct {
		Module      string `json:"module"`
		MsgUuid     string `json:"msg_uuid"`
		Route       string `json:"route"`
		ContentType string `json:"content_type"`
		Timestamp   string `json:"timestamp"`
		Status      int    `json:"status"`
		Reason      string `json:"reason"`
	}{
		Module:      response.metadata.Module,
		MsgUuid:     response.metadata.MsgUuid,
		Route:       response.metadata.Route,
		ContentType: response.metadata.ContentType,
		Timestamp:   response.metadata.Timestamp,
		Status:      response.metadata.Status,
		Reason:      response.metadata.Reason,
	})
	if err != nil {
		return nil, fmt.Errorf("metadata serialization failed: %w", err)
	}
	metadataString := string(metadataBytes)

	return encodeCommon(&response.protocolHeader, metadataString, response.content)
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

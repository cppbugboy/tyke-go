package common

import (
	"encoding/binary"
	"testing"
)

func TestProtocolMagic(t *testing.T) {
	if ProtocolMagic != [4]byte{'T', 'Y', 'K', 'E'} {
		t.Errorf("ProtocolMagic mismatch: got %v, want [TYKE]", ProtocolMagic)
	}
}

func TestContentTypeMap(t *testing.T) {
	tests := []struct {
		ct   ContentType
		want string
	}{
		{ContentTypeText, "text"},
		{ContentTypeJson, "json"},
		{ContentTypeBinary, "binary"},
	}
	for _, tt := range tests {
		got := ContentTypeMap[tt.ct]
		if got != tt.want {
			t.Errorf("ContentTypeMap[%v] = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestContentTypeReverseMap(t *testing.T) {
	tests := []struct {
		s    string
		want ContentType
	}{
		{"text", ContentTypeText},
		{"json", ContentTypeJson},
		{"binary", ContentTypeBinary},
	}
	for _, tt := range tests {
		got := ContentTypeReverseMap[tt.s]
		if got != tt.want {
			t.Errorf("ContentTypeReverseMap[%q] = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestMessageTypeValues(t *testing.T) {
	tests := []struct {
		mt   MessageType
		want int
	}{
		{MessageTypeNone, 0},
		{MessageTypeRequest, 1},
		{MessageTypeRequestAsync, 2},
		{MessageTypeRequestAsyncFunc, 3},
		{MessageTypeRequestAsyncFuture, 4},
		{MessageTypeResponse, 5},
		{MessageTypeResponseAsync, 6},
		{MessageTypeResponseAsyncFunc, 7},
		{MessageTypeResponseAsyncFuture, 8},
	}
	for _, tt := range tests {
		if int(tt.mt) != tt.want {
			t.Errorf("MessageType value = %d, want %d", tt.mt, tt.want)
		}
	}
}

func TestProtocolHeaderSerialization(t *testing.T) {
	header := ProtocolHeader{
		Magic:       ProtocolMagic,
		MsgType:     MessageTypeRequest,
		Reserved:    [3]uint32{0, 0, 0},
		MetadataLen: 100,
		ContentLen:  200,
	}

	buf := make([]byte, ProtocolHeaderSize)
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

	if len(buf) != ProtocolHeaderSize {
		t.Errorf("serialized header size = %d, want %d", len(buf), ProtocolHeaderSize)
	}

	if buf[0] != 'T' || buf[1] != 'Y' || buf[2] != 'K' || buf[3] != 'E' {
		t.Errorf("magic bytes mismatch: got %v", buf[:4])
	}

	gotMsgType := binary.LittleEndian.Uint32(buf[4:8])
	if gotMsgType != 1 {
		t.Errorf("msg_type = %d, want 1", gotMsgType)
	}

	gotMetaLen := binary.LittleEndian.Uint32(buf[20:24])
	if gotMetaLen != 100 {
		t.Errorf("metadata_len = %d, want 100", gotMetaLen)
	}

	gotContentLen := binary.LittleEndian.Uint32(buf[24:28])
	if gotContentLen != 200 {
		t.Errorf("content_len = %d, want 200", gotContentLen)
	}
}

package ipc

import (
	"bytes"
	"testing"
)

func TestBuildFrame_RoundTrip(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03}
	frame := BuildFrame(MsgData, payload)
	if len(frame) != 4+1+len(payload) {
		t.Fatalf("frame length = %d, want %d", len(frame), 4+1+len(payload))
	}
	buf := frame
	ft, ext, err := ExtractFrame(&buf)
	if err != nil {
		t.Fatalf("ExtractFrame failed: %v", err)
	}
	if ft != MsgData {
		t.Fatalf("frame type = 0x%02X, want 0x%02X", ft, MsgData)
	}
	if !bytes.Equal(ext, payload) {
		t.Fatalf("payload mismatch: got %v, want %v", ext, payload)
	}
	if len(buf) != 0 {
		t.Fatalf("buffer not fully consumed, remaining = %d", len(buf))
	}
}

func TestExtractFrame_BufferTooSmall(t *testing.T) {
	buf := []byte{0x00, 0x00}
	_, _, err := ExtractFrame(&buf)
	if err == nil {
		t.Fatal("expected error for buffer < 5 bytes, got nil")
	}
}

func TestExtractFrame_TotalLenTooSmall(t *testing.T) {
	buf := []byte{0x00, 0x00, 0x00, 0x00, 0x03}
	_, _, err := ExtractFrame(&buf)
	if err == nil {
		t.Fatal("expected error for totalLen < 1, got nil")
	}
}

func TestExtractFrame_PayloadTooLarge(t *testing.T) {
	totalLen := MaxFramePayloadLen + 2
	buf := make([]byte, 4)
	encodeU32(totalLen, &buf)
	buf = append(buf, MsgData)
	_, _, err := ExtractFrame(&buf)
	if err == nil {
		t.Fatal("expected error for payload > MaxFramePayloadLen, got nil")
	}
}

func TestExtractFrame_IncompleteBuffer(t *testing.T) {
	buf := []byte{0x0A, 0x00, 0x00, 0x00, MsgData, 0x01}
	_, _, err := ExtractFrame(&buf)
	if err == nil {
		t.Fatal("expected error for incomplete buffer, got nil")
	}
}

func TestParseFragmentHeader_Valid(t *testing.T) {
	totalSize := uint32(1024)
	offset := uint32(0)
	chunk := make([]byte, 100)
	payload := BuildFragmentPayload(totalSize, offset, chunk)

	ts, off, ch, err := ParseFragmentHeader(payload)
	if err != nil {
		t.Fatalf("ParseFragmentHeader failed: %v", err)
	}
	if ts != totalSize {
		t.Fatalf("totalSize = %d, want %d", ts, totalSize)
	}
	if off != offset {
		t.Fatalf("offset = %d, want %d", off, offset)
	}
	if len(ch) != len(chunk) {
		t.Fatalf("chunk length = %d, want %d", len(ch), len(chunk))
	}
}

func TestParseFragmentHeader_PayloadTooSmall(t *testing.T) {
	_, _, _, err := ParseFragmentHeader([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Fatal("expected error for payload < FragmentHeaderSize, got nil")
	}
}

func TestParseFragmentHeader_TotalSizeZero(t *testing.T) {
	payload := BuildFragmentPayload(0, 0, []byte{0x01})
	_, _, _, err := ParseFragmentHeader(payload)
	if err == nil {
		t.Fatal("expected error for totalSize == 0, got nil")
	}
}

func TestParseFragmentHeader_TotalSizeExceedsMax(t *testing.T) {
	payload := BuildFragmentPayload(MaxMessageSize+1, 0, []byte{0x01})
	_, _, _, err := ParseFragmentHeader(payload)
	if err == nil {
		t.Fatal("expected error for totalSize > MaxMessageSize, got nil")
	}
}

func TestParseFragmentHeader_TotalSizeAtMax(t *testing.T) {
	chunk := []byte{0x01}
	payload := BuildFragmentPayload(MaxMessageSize, 0, chunk)
	ts, _, _, err := ParseFragmentHeader(payload)
	if err != nil {
		t.Fatalf("expected success for totalSize == MaxMessageSize, got: %v", err)
	}
	if ts != MaxMessageSize {
		t.Fatalf("totalSize = %d, want %d", ts, MaxMessageSize)
	}
}

func TestParseFragmentHeader_OffsetExceedsTotalSize(t *testing.T) {
	payload := BuildFragmentPayload(100, 200, []byte{0x01})
	_, _, _, err := ParseFragmentHeader(payload)
	if err == nil {
		t.Fatal("expected error for offset > totalSize, got nil")
	}
}

func TestParseFragmentHeader_ChunkExceedsTotalSize(t *testing.T) {
	chunk := make([]byte, 200)
	payload := BuildFragmentPayload(100, 0, chunk)
	_, _, _, err := ParseFragmentHeader(payload)
	if err == nil {
		t.Fatal("expected error for offset+chunk > totalSize, got nil")
	}
}

func TestParseFragmentHeader_ChunkAtExactBoundary(t *testing.T) {
	chunk := make([]byte, 100)
	payload := BuildFragmentPayload(100, 0, chunk)
	_, _, ch, err := ParseFragmentHeader(payload)
	if err != nil {
		t.Fatalf("expected success at exact boundary, got: %v", err)
	}
	if len(ch) != 100 {
		t.Fatalf("chunk length = %d, want 100", len(ch))
	}
}

func TestValidateOffset_Valid(t *testing.T) {
	r := &FragmentReassembly{Total: 1000, NextOffset: 0}
	if !r.ValidateOffset(0, 100) {
		t.Fatal("expected valid offset=0 chunk=100")
	}
}

func TestValidateOffset_OffsetMismatch(t *testing.T) {
	r := &FragmentReassembly{Total: 1000, NextOffset: 50}
	if r.ValidateOffset(0, 100) {
		t.Fatal("expected false for offset mismatch")
	}
}

func TestValidateOffset_ChunkExceedsTotal(t *testing.T) {
	r := &FragmentReassembly{Total: 1000, NextOffset: 0}
	if r.ValidateOffset(0, 1001) {
		t.Fatal("expected false for offset+chunk > total")
	}
}

func TestValidateOffset_LargeValuesNoOverflow(t *testing.T) {
	r := &FragmentReassembly{Total: MaxMessageSize, NextOffset: MaxMessageSize - 100}
	if !r.ValidateOffset(MaxMessageSize-100, 100) {
		t.Fatal("expected valid for large values at boundary")
	}
	if r.ValidateOffset(MaxMessageSize-100, 101) {
		t.Fatal("expected false for large values exceeding boundary")
	}
}

func TestFragmentReassembly_ResetAndComplete(t *testing.T) {
	var r FragmentReassembly
	r.Reset(100)
	if r.Total != 100 || r.Received != 0 || r.NextOffset != 0 {
		t.Fatalf("Reset state incorrect: Total=%d Received=%d NextOffset=%d", r.Total, r.Received, r.NextOffset)
	}
	if r.IsComplete() {
		t.Fatal("expected not complete after Reset")
	}
	r.Received = 100
	if !r.IsComplete() {
		t.Fatal("expected complete when Received == Total")
	}
}

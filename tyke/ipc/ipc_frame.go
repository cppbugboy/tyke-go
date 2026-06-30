// Package ipc 帧解析与分片重组。
// 本文件提供 IPC 传输层的分帧能力（与加密无关）。
// 自 2026-06 起，IPC 传输不再使用加密，所有帧载荷为明文。
package ipc

import "fmt"

const (
	MsgData         byte = 0x03
	MsgDataFragment byte = 0x04

	MaxFramePayloadLen uint32 = 16 * 1024 * 1024
	FragmentChunkSize  uint32 = 64 * 1024
	FragmentHeaderSize uint32 = 8
	// MaxMessageSize 限制分片重组后的逻辑消息总大小，防止恶意 totalSize 触发 OOM。
	MaxMessageSize uint32 = 64 * 1024 * 1024
)

func encodeU32(val uint32, out *[]byte) {
	*out = append(*out, byte(val&0xFF), byte((val>>8)&0xFF), byte((val>>16)&0xFF), byte((val>>24)&0xFF))
}

func decodeU32(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}

// BuildFrame 构建帧：[4B total_len(LE)][1B frame_type][payload]。
func BuildFrame(frameType byte, payload []byte) []byte {
	var frame []byte
	totalLen := uint32(1 + len(payload))
	encodeU32(totalLen, &frame)
	frame = append(frame, frameType)
	frame = append(frame, payload...)
	return frame
}

// ExtractFrame 从 buffer 起始处提取一帧，返回帧类型与载荷，并从 buffer 移除已提取数据。
func ExtractFrame(buffer *[]byte) (byte, []byte, error) {
	if len(*buffer) < 5 {
		return 0, nil, fmt.Errorf("buffer too small for frame header")
	}
	totalLen := decodeU32(*buffer)
	if totalLen < 1 {
		*buffer = nil
		return 0, nil, fmt.Errorf("invalid frame: total_len too small: %d < 1", totalLen)
	}
	if totalLen > MaxFramePayloadLen+1 {
		*buffer = nil
		return 0, nil, fmt.Errorf("frame payload too large: %d > %d", totalLen, MaxFramePayloadLen+1)
	}
	if uint32(len(*buffer)) < 4+totalLen {
		return 0, nil, fmt.Errorf("buffer incomplete: expected %d bytes, got %d", 4+totalLen, len(*buffer))
	}
	frameType := (*buffer)[4]
	payload := make([]byte, totalLen-1)
	copy(payload, (*buffer)[5:4+totalLen])
	*buffer = (*buffer)[4+totalLen:]
	return frameType, payload, nil
}

// FragmentReassembly 跟踪分片消息的重组状态。
type FragmentReassembly struct {
	Buffer     []byte
	Total      uint32
	Received   uint32
	NextOffset uint32
}

func (r *FragmentReassembly) Reset(totalSize uint32) {
	r.Buffer = make([]byte, totalSize)
	r.Total = totalSize
	r.Received = 0
	r.NextOffset = 0
}

func (r *FragmentReassembly) IsComplete() bool {
	return r.Received == r.Total && r.Total > 0
}

func (r *FragmentReassembly) ValidateOffset(offset uint32, chunkLen int) bool {
	if offset != r.NextOffset {
		return false
	}
	// 使用 uint64 比较，避免 32-bit 平台 int 溢出导致校验失效
	if uint64(offset)+uint64(chunkLen) > uint64(r.Total) {
		return false
	}
	return true
}

// BuildFragmentPayload 构建分片帧载荷：[4B total_size][4B offset][chunk]。
func BuildFragmentPayload(totalSize uint32, offset uint32, chunk []byte) []byte {
	var payload []byte
	encodeU32(totalSize, &payload)
	encodeU32(offset, &payload)
	payload = append(payload, chunk...)
	return payload
}

// ParseFragmentHeader 解析分片帧载荷头，返回 total_size、offset 与数据块。
// 同时校验 totalSize 与 offset 的合法性，防止恶意帧触发 OOM 或越界。
func ParseFragmentHeader(payload []byte) (totalSize uint32, offset uint32, chunk []byte, err error) {
	if uint32(len(payload)) < FragmentHeaderSize {
		return 0, 0, nil, fmt.Errorf("fragment payload too small: %d < %d", len(payload), FragmentHeaderSize)
	}
	totalSize = decodeU32(payload[0:4])
	offset = decodeU32(payload[4:8])
	if totalSize == 0 || totalSize > MaxMessageSize {
		return 0, 0, nil, fmt.Errorf("invalid fragment totalSize: %d (must be in [1, %d])", totalSize, MaxMessageSize)
	}
	if offset > totalSize {
		return 0, 0, nil, fmt.Errorf("fragment offset %d > totalSize %d", offset, totalSize)
	}
	chunk = payload[FragmentHeaderSize:]
	if uint64(offset)+uint64(len(chunk)) > uint64(totalSize) {
		return 0, 0, nil, fmt.Errorf("fragment chunk exceeds totalSize: offset=%d chunk=%d total=%d", offset, len(chunk), totalSize)
	}
	return totalSize, offset, chunk, nil
}

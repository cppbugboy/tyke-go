# Tyke IPC Protocol Specification v1

This document is the **single source of truth** for the Tyke IPC protocol.
Both the C++ and Go implementations **must** conform to the values defined here.

---

## 1. Protocol Header

```
Offset  Size  Field          Type        Description
------  ----  -------------  ----------  -------------------------------------------
0x00    4     magic          byte[4]     Fixed value: {'T','Y','K','E'} (0x54594B45)
0x04    4     msg_type       uint32_le   MessageType enum value
0x08    12    reserved       uint32[3]   Reserved, must be zero
0x14    4     metadata_len   uint32_le   Length of the metadata JSON section (bytes)
0x18    4     content_len    uint32_le   Length of the binary content section (bytes)
------  ----  -------------  ----------  -------------------------------------------
Total:  28 bytes
```

Wire format: `[ProtocolHeader 28B][Metadata JSON metadata_len B][Content Binary content_len B]`

All multi-byte integers are **little-endian**.

## 2. Message Types

| Name                       | Value | Description                              |
| -------------------------- | ----- | ---------------------------------------- |
| NONE                       | 0     | Uninitialized / invalid                  |
| REQUEST                    | 1     | Synchronous request                      |
| REQUEST_ASYNC              | 2     | Async request (fire-and-forget)          |
| REQUEST_ASYNC_FUNC         | 3     | Async request with callback              |
| REQUEST_ASYNC_FUTURE       | 4     | Async request with future/promise        |
| RESPONSE                   | 5     | Synchronous response                     |
| RESPONSE_ASYNC             | 6     | Async response (fire-and-forget)         |
| RESPONSE_ASYNC_FUNC        | 7     | Async response for callback              |
| RESPONSE_ASYNC_FUTURE      | 8     | Async response for future/promise        |

## 3. Status Codes

| Name             | Value | Description          |
| ---------------- | ----- | -------------------- |
| NONE             | 0     | No status            |
| SUCCESS          | 1     | Operation succeeded  |
| FAILURE          | 2     | General failure      |
| TIMEOUT          | 3     | Operation timed out  |
| METADATA_ERROR   | 4     | Metadata error       |
| CONTENT_ERROR    | 5     | Content error        |
| ROUTE_ERROR      | 6     | Route not found      |
| MODULE_ERROR     | 7     | Module not supported |
| INTERNAL_ERROR   | 8     | Internal error       |
| UNAVAILABLE      | 9     | Service unavailable  |
| UNKNOWN_ERROR    | 10    | Unknown error        |

## 4. Content Types

| Name   | String Value | Enum Index |
| ------ | ------------ | ---------- |
| TEXT   | "text"       | 0          |
| JSON   | "json"       | 1          |
| BINARY | "binary"     | 2          |

## 5. Frame Format (Encryption Layer)

```
Offset  Size           Field       Type        Description
------  -------------  ----------  ----------  -------------------------------------------
0x00    4              total_len   uint32_le   1 + len(payload)
0x04    1              frame_type  uint8       HandshakeInit(0x01) / HandshakeResp(0x02) / Data(0x03)
0x05    total_len - 1  payload     byte[]      Frame payload
------  -------------  ----------  ----------  -------------------------------------------
```

Max frame payload: **16 MiB** (16777216 bytes)

### Frame Types

| Name            | Value | Description                         |
| --------------- | ----- | ----------------------------------- |
| HANDSHAKE_INIT  | 0x01  | Client ECDH public key (SPKI DER)   |
| HANDSHAKE_RESP  | 0x02  | Server ECDH public key (SPKI DER)   |
| DATA            | 0x03  | AES-256-GCM encrypted data          |

## 6. Cryptographic Parameters

| Parameter              | Value                                    |
| ---------------------- | ---------------------------------------- |
| Key Exchange           | ECDH over P-256 (prime256v1 / secp256r1) |
| Key Derivation         | HKDF-SHA256                              |
| HKDF Salt              | `tyke-v1-hkdf-salt` (ASCII, 17 bytes)   |
| HKDF Info              | `tyke-v1-aes256-key` (ASCII, 17 bytes)  |
| Derived Key Length     | 32 bytes (AES-256)                       |
| Encryption             | AES-256-GCM                              |
| IV Length              | 12 bytes                                 |
| Authentication Tag     | 16 bytes                                 |
| IV Structure           | [4B random prefix][8B big-endian counter]|
| Public Key Encoding    | SPKI DER (X.509 SubjectPublicKeyInfo)    |

### Encrypted Data Format

```
[IV 12B][Ciphertext NB][Auth Tag 16B]
```

## 7. Metadata JSON Fields

| Key           | Type   | Description                          |
| ------------- | ------ | ------------------------------------ |
| module        | string | Module name                          |
| route         | string | Route path (e.g., "/api/user/login") |
| msg_uuid      | string | Message UUID (v4)                    |
| async_uuid    | string | Async listener UUID                  |
| content_type  | string | "text" / "json" / "binary"           |
| timestamp     | string | ISO 8601 timestamp                   |
| timeout       | uint64 | Timeout in milliseconds              |
| status        | int    | Status code (response only)          |
| reason        | string | Status reason (response only)        |

Additional keys are stored in a `headers` map and passed through as-is.

## 8. IPC Transport

### Windows
- **Transport**: Named Pipes (`\\.\pipe\<server_name>`)
- **I/O Model**: Overlapped I/O (IOCP)
- **Handshake Timeout**: 5000ms default

### Linux
- **Transport**: Unix Domain Sockets (abstract namespace `@tyke_<server_name>`)
- **I/O Model**: epoll
- **Handshake Timeout**: 5000ms default

## 9. Default Constants

| Constant                    | Value    |
| --------------------------- | -------- |
| Default Timeout             | 5000 ms  |
| Default Buffer Size         | 4096 B   |
| Default Thread Pool Size    | 4        |
| Default Max Connections     | 4        |
| Default Idle Timeout        | 30000 ms |
| Default Stub Timeout        | 30000 ms |
| Protocol Header Size        | 28 B     |

---

## Change Log

| Version | Date       | Changes                          |
| ------- | ---------- | -------------------------------- |
| v1      | 2026-04-26 | Initial protocol specification   |

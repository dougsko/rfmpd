# RF Microblog Protocol v0.4 Specification

This document defines the RFMP wire format, encoding rules, and protocol behavior
independent of any particular implementation language.

---

## 1. Overview

RFMP is a decentralized, append-only microblogging protocol for packet radio networks.
Messages propagate via AX.25 UI (unconnected) frames over half-duplex radio links.

**Layer stack:**
```
RFMP payload (Protocol Buffers binary)
  inside
AX.25 UI frame (info field)
  inside
KISS framing (TCP or serial to TNC)
```

---

## 2. RFMP Frame Wire Format

### 2.1 General Structure

Every RFMP frame is a Protocol Buffers v3 binary-encoded `Frame` message:

```protobuf
syntax = "proto3";
package rfmp;

message Frame {
  oneof payload {
    Msg msg = 1;
    Frag frag = 2;
    Svec svec = 3;
  }
}
```

The `oneof` discriminator identifies the frame type without a magic byte prefix.
A single `proto.Marshal` / `proto.Unmarshal` handles encoding and dispatch.

### 2.2 Binary Encoding

All frames are binary (not text). The protobuf wire format provides:
- Variable-length integer encoding (varints) for small numbers
- Length-delimited fields for strings and bytes
- No explicit field ordering requirement

---

## 3. Frame Types

### 3.1 MSG (Message)

```protobuf
message Msg {
  bytes id = 1;           // 6 bytes — message identifier
  string from_node = 2;   // sender callsign (e.g. "N0CALL-1")
  uint32 timestamp = 3;   // Unix epoch seconds (UTC)
  string channel = 4;     // channel name (ASCII, no uppercase)
  bytes reply_to = 5;     // 6 bytes or empty (no reply)
  string body = 6;        // UTF-8 message text
  uint32 seq = 7;         // per-node sequence number; 0 = not set
}
```

| Field | Type | Constraints |
|-------|------|-------------|
| `id` | bytes | Exactly 6 bytes |
| `from_node` | string | Radio callsign |
| `timestamp` | uint32 | Unix epoch seconds (UTC) |
| `channel` | string | ASCII, no uppercase letters |
| `reply_to` | bytes | 6 bytes if replying, empty otherwise |
| `body` | string | UTF-8 |
| `seq` | uint32 | >= 1 if set; 0 means unset |

### 3.2 FRAG (Fragment)

```protobuf
message Frag {
  bytes msg_id = 1;       // 6 bytes — ID of the message being fragmented
  uint32 idx = 2;         // fragment index (0-based)
  uint32 total = 3;       // total number of fragments
  bytes data = 4;         // raw protobuf bytes of the Msg
}
```

| Field | Type | Constraints |
|-------|------|-------------|
| `msg_id` | bytes | 6 bytes |
| `idx` | uint32 | 0 <= idx < total |
| `total` | uint32 | > 0 |
| `data` | bytes | Raw serialized `Msg` chunk (NOT base64) |

**What `data` contains:** The `Msg` is serialized to protobuf bytes, split into
chunks, and each chunk placed directly in a FRAG's `data` field. No base64 encoding.

### 3.3 SVEC (State Vector)

```protobuf
message Svec {
  string from_node = 1;   // broadcasting node's callsign
  map<string, uint32> vector = 2;  // callsign → highest contiguous seq
}
```

| Field | Type | Constraints |
|-------|------|-------------|
| `from_node` | string | Non-empty |
| `vector` | map | All values >= 0 |

---

## 4. Message ID Generation

```
input  = from_node (UTF-8 bytes)
       + timestamp (4 bytes, big-endian uint32 epoch)
       + body (UTF-8 bytes)
hash   = SHA-256(input)
id     = first 6 bytes of hash
```

- **Wire format:** 6 raw bytes
- **Display format:** 12 lowercase hex characters
- **Collision resistance:** 48 bits (sufficient for low-volume packet radio)

The same body from the same node at the same second produces the same ID
(intentional deduplication for retransmissions).

---

## 5. Timestamp Format

**Wire format:** `uint32` Unix epoch seconds (UTC). Valid range: 0 to 4,294,967,295
(year 2106).

**API/Display format:** ISO 8601 `YYYYMMDDTHHMMSSZ` (16 characters, UTC).
Conversion between epoch and ISO happens at the protocol boundary.

---

## 6. Synchronization (SVEC Protocol)

Nodes periodically broadcast SVEC frames containing their vector clock: the highest
contiguous sequence number per originating node.

**On receiving an SVEC frame:**

For each node in the union of local and remote vectors:
1. If `local_seq > remote_seq`: the remote is missing messages.
2. Fetch all local messages from that node with `seq > remote_seq`.
3. Queue those messages for transmission (broadcast to the network).
4. Skip messages whose `from_node` equals the SVEC sender (prevents echo loops).

**Sequence numbers:**
- Each node maintains a monotonic counter for its own outgoing messages.
- New messages are assigned `MAX(seq) + 1` for the local callsign.
- The vector clock reports the **highest contiguous sequence** per node.

**Empty vectors:**
- A node with no messages broadcasts an SVEC with an empty vector map.
- On receiving an empty SVEC, other nodes push all their known messages.

---

## 7. Fragmentation and Reassembly

### Fragmentation (sender side)

1. Serialize the `Msg` to protobuf bytes (raw, not wrapped in `Frame`).
2. If `len(serialized) <= fragment_threshold`: wrap in `Frame{msg: ...}` and send.
3. Otherwise, split serialized bytes into chunks of `fragment_size` bytes.
4. Each chunk becomes a `Frag` with sequential `idx` values and the same `total`.

**Fragment size calculation:**
```
fragment_size = threshold - 15  (protobuf Frag overhead)
total_fragments = ceil(len(serialized_msg) / fragment_size)
```

### Reassembly (receiver side)

1. On receiving a FRAG, look up or create a collector keyed by `msg_id`.
2. Reject if `frag.total != collector.total` (inconsistency).
3. Reject if `frag.idx` already received (duplicate).
4. Store fragment data indexed by `idx`.
5. When all fragments received:
   - Concatenate in index order (0, 1, 2, ...).
   - Unmarshal as a `Msg` protobuf message.
   - If unmarshal fails, discard.
6. Collector timeout: 5 minutes from first fragment received.

---

## 8. AX.25 Framing

### 8.1 Address Format (7 bytes)

```
Bytes 0-5: callsign characters, each left-shifted 1 bit, space-padded to 6
Byte 6:    SSID byte
```

**Callsign encoding:**
- Uppercase only (6 chars max, right-padded with spaces)
- Each byte: `encoded = ord(character) << 1`

**SSID byte:**
```
Bit 7:   0 (reserved)
Bit 6:   1 (reserved, always set)
Bit 5:   1 (reserved, always set)
Bits 4-1: SSID value (0-15), shifted left 1
Bit 0:   Address extension bit (1 = last address, 0 = more follow)
```

Formula: `ssid_byte = 0x60 | (ssid << 1) | (1 if last else 0)`

### 8.2 UI Frame Structure

```
[Destination: 7 bytes][Source: 7 bytes][Digipeaters: 7 bytes each, 0+][Control: 0x03][PID: 0xF0][Info: N bytes]
```

- Control `0x03` = UI (Unnumbered Information) frame
- PID `0xF0` = No Layer 3 protocol
- Default destination for RFMP: `RFMP` (callsign)
- The last address in the address field has bit 0 of its SSID byte set to 1

### 8.3 String Representation

- SSID 0: just the callsign (e.g., `N0CALL`)
- SSID 1-15: callsign-SSID (e.g., `N0CALL-1`)

---

## 9. KISS Framing

### 9.1 Special Bytes

| Name | Value | Purpose |
|------|-------|---------|
| FEND | 0xC0 | Frame delimiter |
| FESC | 0xDB | Escape prefix |
| TFEND | 0xDC | Escaped FEND |
| TFESC | 0xDD | Escaped FESC |

### 9.2 Frame Structure

```
FEND [command_byte] [escaped payload] FEND
```

**Command byte:** `(port << 4) | command_code`

| Command | Code |
|---------|------|
| DATA_FRAME | 0x00 |
| TX_DELAY | 0x01 |
| PERSISTENCE | 0x02 |
| SLOT_TIME | 0x03 |
| TX_TAIL | 0x04 |
| FULL_DUPLEX | 0x05 |
| SET_HARDWARE | 0x06 |
| RETURN | 0x0F |

### 9.3 Byte Stuffing

On encode (applied to command byte + payload):

| Byte | Replaced with |
|------|--------------|
| 0xC0 | 0xDB 0xDC |
| 0xDB | 0xDB 0xDD |

On decode:

| Sequence | Decoded as |
|----------|-----------|
| 0xDB 0xDC | 0xC0 |
| 0xDB 0xDD | 0xDB |
| 0xDB + other | Invalid frame |
| 0xDB at end | Invalid frame |

### 9.4 Protocol Behavior

- Only DATA_FRAME (0x00) frames carry AX.25 data; all others are TNC control commands.
- Partial frames are buffered until a complete FEND-to-FEND sequence arrives.

---

## 10. Deduplication

Messages are deduplicated by their `id` field (6 bytes, primary key in storage).
Fragments are deduplicated by `(msg_id, idx)` pair.

A time-limited "seen cache" provides fast dedup without querying the message store.
The cache has a configurable TTL (default: 1 hour). The persistent message store
provides ultimate dedup regardless of cache expiry.

---

## 11. Wire Size Budget

At 1200 baud VHF, each byte costs 8ms of airtime. At 300 baud HF, 26ms/byte.
Typical frame sizes with protobuf encoding:

| Frame | Typical Size |
|-------|-------------|
| MSG (short body) | 40-50 bytes |
| MSG (140 chars) | ~180 bytes |
| FRAG (per fragment) | ~200 bytes (threshold) |
| SVEC (5 nodes) | ~70 bytes |

These represent 30-50% savings over the previous text-based wire format (v0.3).

---

## 12. Conformance Testing

The file `test_vectors.json` in this repository contains machine-readable test vectors
covering protocol layers: protobuf encoding/decoding, message ID generation,
fragmentation, KISS byte-stuffing, and AX.25 address encoding.

An implementation is conformant if it:
1. Produces byte-identical output for all encoding vectors.
2. Correctly decodes all valid vectors back to their specified field values.
3. Rejects all invalid vectors listed in the `validation` section.

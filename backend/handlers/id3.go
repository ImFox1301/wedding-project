package handlers

import (
	"bytes"
	"os"
	"strings"
)

// ReadAudioTags reads Title and Artist from ID3v2 (MP3) or ID3v1 tags.
// Returns empty strings if tags are absent or the format is unsupported.
func ReadAudioTags(path string) (title, artist string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// ── ID3v2 ───────────────────────────────────────────────────────────────
	hdr := make([]byte, 10)
	if n, err := f.Read(hdr); n == 10 && err == nil && string(hdr[0:3]) == "ID3" {
		ver := hdr[3] // 3 = v2.3, 4 = v2.4
		// syncsafe integer: high bit of each byte is ignored
		tagSize := int(hdr[6])<<21 | int(hdr[7])<<14 | int(hdr[8])<<7 | int(hdr[9])
		tagData := make([]byte, tagSize+10)
		if _, err := f.ReadAt(tagData, 0); err == nil {
			title, artist = parseID3v2Frames(tagData[10:], ver)
		}
		if title != "" || artist != "" {
			return
		}
	}

	// ── ID3v1 fallback ──────────────────────────────────────────────────────
	fi, err := f.Stat()
	if err != nil || fi.Size() < 128 {
		return
	}
	tag := make([]byte, 128)
	if _, err := f.ReadAt(tag, fi.Size()-128); err == nil && string(tag[0:3]) == "TAG" {
		title = trimNull(tag[3:33])
		artist = trimNull(tag[33:63])
	}
	return
}

func trimNull(b []byte) string {
	return strings.TrimRight(strings.TrimRight(string(b), "\x00"), " ")
}

func parseID3v2Frames(data []byte, ver byte) (title, artist string) {
	offset := 0
	for offset+10 < len(data) {
		frameID := string(data[offset : offset+4])
		if frameID == "\x00\x00\x00\x00" {
			break // padding reached
		}

		var frameSize int
		if ver >= 4 {
			// ID3v2.4: syncsafe
			frameSize = int(data[offset+4])<<21 | int(data[offset+5])<<14 |
				int(data[offset+6])<<7 | int(data[offset+7])
		} else {
			// ID3v2.3: plain big-endian
			frameSize = int(data[offset+4])<<24 | int(data[offset+5])<<16 |
				int(data[offset+6])<<8 | int(data[offset+7])
		}
		if frameSize <= 0 || offset+10+frameSize > len(data) {
			break
		}

		payload := data[offset+10 : offset+10+frameSize]
		if len(payload) > 1 {
			text := decodeID3String(payload)
			switch frameID {
			case "TIT2":
				title = text
			case "TPE1":
				artist = text
			}
		}
		offset += 10 + frameSize
	}
	return
}

// decodeID3String decodes an ID3 text payload (first byte = encoding).
func decodeID3String(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	enc := b[0]
	raw := b[1:]

	switch enc {
	case 1, 2: // UTF-16 with BOM (1) or UTF-16 BE (2)
		return decodeUTF16(raw)
	default: // 0 = Latin-1, 3 = UTF-8
		if idx := bytes.IndexByte(raw, 0); idx >= 0 {
			raw = raw[:idx]
		}
		return strings.TrimSpace(string(raw))
	}
}

func decodeUTF16(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	bigEndian := false
	if b[0] == 0xFE && b[1] == 0xFF {
		bigEndian = true
		b = b[2:]
	} else if b[0] == 0xFF && b[1] == 0xFE {
		b = b[2:]
	}
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		var r rune
		if bigEndian {
			r = rune(b[i])<<8 | rune(b[i+1])
		} else {
			r = rune(b[i]) | rune(b[i+1])<<8
		}
		if r == 0 {
			break
		}
		runes = append(runes, r)
	}
	return strings.TrimSpace(string(runes))
}

package main

import (
	"embed"
	"errors"
)

// ReaderEventType describes a parsed MIDI event kind.
type ReaderEventType uint8

const (
	ReaderEventNoteOn ReaderEventType = iota
	ReaderEventNoteOff
	ReaderEventProgramChange
	ReaderEventControlChange
)

// ReaderEvent is a minimal parsed MIDI event with delta time in ticks.
type ReaderEvent struct {
	DeltaTicks uint32
	Type       ReaderEventType
	Channel    uint8
	Data1      uint8
	Data2      uint8
}

// MIDIFile contains the basic SMF header and parsed events from the first track.
type MIDIFile struct {
	Format         uint16
	TrackCount     uint16
	Division       uint16
	Events         []ReaderEvent
	TrackEndOfFile bool
}

// ReadEmbeddedMIDI reads and parses a single embedded Standard MIDI File.
func ReadEmbeddedMIDI(fs embed.FS, path string) (*MIDIFile, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseMIDI(data)
}

// ParseMIDI provides a very small MIDI file parser suitable for TinyGo.
// It parses the SMF header and supported channel events from track 0.
func ParseMIDI(data []byte) (*MIDIFile, error) {
	if len(data) < 14 {
		return nil, errors.New("midi: file too short")
	}
	if string(data[:4]) != "MThd" {
		return nil, errors.New("midi: missing header chunk")
	}

	headerLen := be32(data[4:8])
	if headerLen < 6 || len(data) < int(8+headerLen) {
		return nil, errors.New("midi: invalid header length")
	}

	m := &MIDIFile{
		Format:     be16(data[8:10]),
		TrackCount: be16(data[10:12]),
		Division:   be16(data[12:14]),
	}

	offset := int(8 + headerLen)
	if len(data) < offset+8 {
		return nil, errors.New("midi: missing track chunk")
	}
	if string(data[offset:offset+4]) != "MTrk" {
		return nil, errors.New("midi: first track chunk not found")
	}

	trackLen := int(be32(data[offset+4 : offset+8]))
	offset += 8
	if len(data) < offset+trackLen {
		return nil, errors.New("midi: truncated track chunk")
	}

	events, ended, err := parseTrack(data[offset : offset+trackLen])
	if err != nil {
		return nil, err
	}
	m.Events = events
	m.TrackEndOfFile = ended
	return m, nil
}

func parseTrack(track []byte) ([]ReaderEvent, bool, error) {
	var events []ReaderEvent
	var runningStatus byte
	i := 0
	ended := false

	for i < len(track) {
		delta, next, ok := readVarLen(track, i)
		if !ok {
			return nil, false, errors.New("midi: invalid delta time")
		}
		i = next
		if i >= len(track) {
			return nil, false, errors.New("midi: unexpected end of track")
		}

		status := track[i]
		if status < 0x80 {
			if runningStatus == 0 {
				return nil, false, errors.New("midi: running status without previous status")
			}
			status = runningStatus
		} else {
			i++
			if status < 0xF0 {
				runningStatus = status
			}
		}

		switch {
		case status == 0xFF:
			if i >= len(track) {
				return nil, false, errors.New("midi: truncated meta event")
			}
			metaType := track[i]
			i++
			length, next, ok := readVarLen(track, i)
			if !ok {
				return nil, false, errors.New("midi: invalid meta length")
			}
			i = next
			end := i + int(length)
			if end > len(track) {
				return nil, false, errors.New("midi: truncated meta payload")
			}
			if metaType == 0x2F {
				ended = true
				return events, ended, nil
			}
			i = end
		case status == 0xF0 || status == 0xF7:
			length, next, ok := readVarLen(track, i)
			if !ok {
				return nil, false, errors.New("midi: invalid sysex length")
			}
			i = next + int(length)
			if i > len(track) {
				return nil, false, errors.New("midi: truncated sysex event")
			}
		case status&0xF0 == 0x80 || status&0xF0 == 0x90 || status&0xF0 == 0xB0:
			if i+2 > len(track) {
				return nil, false, errors.New("midi: truncated channel event")
			}
			evType := ReaderEventNoteOn
			switch status & 0xF0 {
			case 0x80:
				evType = ReaderEventNoteOff
			case 0x90:
				evType = ReaderEventNoteOn
			case 0xB0:
				evType = ReaderEventControlChange
			}
			events = append(events, ReaderEvent{
				DeltaTicks: delta,
				Type:       evType,
				Channel:    status & 0x0F,
				Data1:      track[i] & 0x7F,
				Data2:      track[i+1] & 0x7F,
			})
			i += 2
		case status&0xF0 == 0xC0:
			if i >= len(track) {
				return nil, false, errors.New("midi: truncated program change")
			}
			events = append(events, ReaderEvent{
				DeltaTicks: delta,
				Type:       ReaderEventProgramChange,
				Channel:    status & 0x0F,
				Data1:      track[i] & 0x7F,
			})
			i++
		case status&0xF0 == 0xA0 || status&0xF0 == 0xE0:
			if i+2 > len(track) {
				return nil, false, errors.New("midi: truncated 2-byte channel event")
			}
			i += 2
		case status&0xF0 == 0xD0:
			if i >= len(track) {
				return nil, false, errors.New("midi: truncated 1-byte channel event")
			}
			i++
		default:
			return nil, false, errors.New("midi: unsupported status byte")
		}
	}

	return events, ended, nil
}

func readVarLen(data []byte, start int) (uint32, int, bool) {
	var value uint32
	for j := 0; j < 4 && start+j < len(data); j++ {
		b := data[start+j]
		value = (value << 7) | uint32(b&0x7F)
		if b&0x80 == 0 {
			return value, start + j + 1, true
		}
	}
	return 0, start, false
}

func be16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

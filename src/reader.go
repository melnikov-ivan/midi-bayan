package main

import (
	"embed"
	"time"
)

//go:embed mid
var midiFS embed.FS

var midiFileNames = [...]string{
	"cernyj-kot-sutkin.mid",
	"demo.mid",
	"razgovor_v_poezde.mid",
}

var midiPlaying bool

// PlayMIDI starts playback of the file at the given index; if already playing, stops it first.
func PlayMIDI(fileIndex byte) {
	if midiPlaying {
		StopMIDI()
		return
	}
	if int(fileIndex) >= len(midiFileNames) {
		println("midi_play: invalid index:", fileIndex)
		return
	}
	name := midiFileNames[fileIndex]
	data, err := midiFS.ReadFile("mid/" + name)
	if err != nil {
		println("midi_play: file not found:", name)
		return
	}
	midiPlaying = true
	go PlayMIDIFile(data)
}

// StopMIDI halts playback; running tracks exit at the next tick check.
func StopMIDI() {
	if !midiPlaying {
		return
	}
	midiPlaying = false
	SendAllNotesOff()
}

// PlayMIDIFile parses the given MIDI data and sends events into EventChannel.
// Format 0: single track. Format 1: all tracks run concurrently (goroutine per track).
// Format 2: tracks run sequentially. Tempo meta events on any track affect all tracks.
func PlayMIDIFile(data []byte) {
	defer StopMIDI()
	d := data
	if len(d) < 14 {
		return
	}
	if d[0] != 'M' || d[1] != 'T' || d[2] != 'h' || d[3] != 'd' {
		return
	}

	format := uint16(d[8])<<8 | uint16(d[9])
	numTracks := int(uint16(d[10])<<8 | uint16(d[11]))
	ppqn := uint32(d[12])<<8 | uint32(d[13])
	if ppqn == 0 {
		ppqn = 96
	}
	if numTracks > 32 {
		numTracks = 32
	}

	type span struct{ pos, end int }
	spans := make([]span, 0, numTracks)
	pos := 14
	for len(spans) < numTracks && pos+8 <= len(d) {
		if d[pos] != 'M' || d[pos+1] != 'T' || d[pos+2] != 'r' || d[pos+3] != 'k' {
			break
		}
		tlen := int(d[pos+4])<<24 | int(d[pos+5])<<16 | int(d[pos+6])<<8 | int(d[pos+7])
		pos += 8
		end := pos + tlen
		if end > len(d) {
			end = len(d)
		}
		spans = append(spans, span{pos, end})
		pos = end
	}
	if len(spans) == 0 {
		return
	}

	tempoUs := uint32(500000) // shared across tracks; updated by tempo meta events
	startMs := time.Now().UnixMilli()

	if format == 1 && len(spans) > 1 {
		done := make(chan struct{}, len(spans))
		for _, s := range spans {
			s := s
			go func() {
				playTrack(d, s.pos, s.end, ppqn, &tempoUs, startMs)
				done <- struct{}{}
			}()
		}
		for range spans {
			<-done
		}
	} else {
		for _, s := range spans {
			playTrack(d, s.pos, s.end, ppqn, &tempoUs, startMs)
		}
	}
}

func playTrack(d []byte, pos, end int, ppqn uint32, tempoUs *uint32, startMs int64) {
	var absoluteMs int64 // milliseconds elapsed from startMs at the current tick position
	var lastStatus byte
	for pos < end {
		if !midiPlaying {
			return
		}
		delta, n := midiVarLen(d[pos:])
		pos += n
		if pos >= end {
			break
		}

		absoluteMs += int64(delta) * int64(*tempoUs) / int64(ppqn) / 1000
		if ms := startMs + absoluteMs - time.Now().UnixMilli(); ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}

		if d[pos]&0x80 != 0 {
			lastStatus = d[pos]
			pos++
		}
		if pos >= end {
			break
		}

		switch {
		case lastStatus == 0xFF: // meta event
			metaType := d[pos]
			pos++
			metaLen, n2 := midiVarLen(d[pos:])
			pos += n2
			if metaType == 0x2F { // end of track
				return
			}
			if metaType == 0x51 && metaLen == 3 && pos+3 <= end {
				*tempoUs = uint32(d[pos])<<16 | uint32(d[pos+1])<<8 | uint32(d[pos+2])
			}
			pos += int(metaLen)

		case lastStatus >= 0xF0: // SysEx
			sysexLen, n2 := midiVarLen(d[pos:])
			pos += n2 + int(sysexLen)

		case lastStatus&0xF0 == 0x80: // NoteOff
			if pos+2 > end {
				return
			}
			EventChannel <- Event{Type: NoteOff, Channel: lastStatus & 0x0F, Note: d[pos]}
			pos += 2

		case lastStatus&0xF0 == 0x90: // NoteOn
			if pos+2 > end {
				return
			}
			ch := lastStatus & 0x0F
			note, vel := d[pos], d[pos+1]
			pos += 2
			if vel == 0 {
				EventChannel <- Event{Type: NoteOff, Channel: ch, Note: note}
			} else {
				EventChannel <- Event{Type: NoteOn, Channel: ch, Note: note, Velocity: vel}
			}

		case lastStatus&0xF0 == 0xA0: // Polyphonic Aftertouch
			pos += 2

		case lastStatus&0xF0 == 0xB0: // Control Change
			if pos+2 > end {
				return
			}
			ch, cc, val := lastStatus&0x0F, d[pos], d[pos+1]
			pos += 2
			switch cc {
			case 7:
				EventChannel <- Event{Type: Volume, Channel: ch, Volume: val}
			case 91:
				EventChannel <- Event{Type: Reverb, Channel: ch, CCValue: val}
			case 93:
				EventChannel <- Event{Type: Chorus, Channel: ch, CCValue: val}
			case 94:
				EventChannel <- Event{Type: Delay, Channel: ch, CCValue: val}
			}

		case lastStatus&0xF0 == 0xC0: // Program Change
			if pos+1 > end {
				return
			}
			EventChannel <- Event{Type: ProgramChange, Channel: lastStatus & 0x0F, Program: d[pos]}
			pos++

		case lastStatus&0xF0 == 0xD0: // Channel Pressure
			pos++

		case lastStatus&0xF0 == 0xE0: // Pitch Bend
			pos += 2

		default:
			pos++
		}
	}
}

// midiVarLen decodes a MIDI variable-length quantity from b.
// Returns the decoded value and number of bytes consumed.
func midiVarLen(b []byte) (val uint32, n int) {
	for n < len(b) && n < 4 {
		c := b[n]
		n++
		val = (val << 7) | uint32(c&0x7F)
		if c&0x80 == 0 {
			return
		}
	}
	return
}

package main

import (
	_ "embed"
	"time"
)

//go:embed mid/demo.mid
var midiFile []byte

// PlayMIDIFile parses the embedded MIDI file and sends events into EventChannel.
// Supports NoteOn/Off, Program Change, and CC (Volume/Reverb/Chorus/Delay).
// Timing follows the file's tempo and ppqn settings.
func PlayMIDIFile() {
	d := midiFile
	if len(d) < 14 {
		return
	}
	if d[0] != 'M' || d[1] != 'T' || d[2] != 'h' || d[3] != 'd' {
		return
	}
	ppqn := uint32(d[12])<<8 | uint32(d[13])
	if ppqn == 0 {
		ppqn = 96
	}

	pos := 14
	for pos+8 <= len(d) {
		if d[pos] != 'M' || d[pos+1] != 'T' || d[pos+2] != 'r' || d[pos+3] != 'k' {
			break
		}
		trackLen := int(d[pos+4])<<24 | int(d[pos+5])<<16 | int(d[pos+6])<<8 | int(d[pos+7])
		pos += 8
		end := pos + trackLen
		if end > len(d) {
			end = len(d)
		}

		tempoUs := uint32(500000) // 120 BPM default
		var lastStatus byte

		for pos < end {
			delta, n := midiVarLen(d[pos:])
			pos += n
			if pos >= end {
				break
			}

			if delta > 0 {
				ms := int64(delta) * int64(tempoUs) / int64(ppqn) / 1000
				if ms > 0 {
					time.Sleep(time.Duration(ms) * time.Millisecond)
				}
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
					pos = end
				} else {
					if metaType == 0x51 && metaLen == 3 && pos+3 <= end {
						tempoUs = uint32(d[pos])<<16 | uint32(d[pos+1])<<8 | uint32(d[pos+2])
					}
					pos += int(metaLen)
				}

			case lastStatus >= 0xF0: // SysEx: skip varlen payload
				sysexLen, n2 := midiVarLen(d[pos:])
				pos += n2 + int(sysexLen)

			case lastStatus&0xF0 == 0x80: // NoteOff
				if pos+2 > end {
					return
				}
				EventChannel <- Event{Type: NoteOn, Channel: lastStatus & 0x0F, Note: d[pos], Velocity: 0}
				pos += 2

			case lastStatus&0xF0 == 0x90: // NoteOn
				if pos+2 > end {
					return
				}
				EventChannel <- Event{Type: NoteOn, Channel: lastStatus & 0x0F, Note: d[pos], Velocity: d[pos+1]}
				pos += 2

			case lastStatus&0xF0 == 0xA0: // Polyphonic Aftertouch — skip
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

			case lastStatus&0xF0 == 0xD0: // Channel Pressure — skip
				pos++

			case lastStatus&0xF0 == 0xE0: // Pitch Bend — skip
				pos += 2

			default:
				pos++
			}
		}
		pos = end
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

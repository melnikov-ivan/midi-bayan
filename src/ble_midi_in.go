package main

const (
	midiInSlotSize  = 20
	midiInQueueSize = 32
)

// Очередь входящих BLE MIDI пакетов (WriteEvent → push, main loop → pop).
var midiInQueue [midiInQueueSize][midiInSlotSize]byte
var midiInQueueLen [midiInQueueSize]uint8
var midiInQueueHead uint8
var midiInQueueTail uint8

// midiInPush копирует пакет в очередь из контекста BLE WriteEvent (без аллокаций).
func midiInPush(value []byte) {
	n := len(value)
	if n > midiInSlotSize {
		n = midiInSlotSize
	}
	if midiInQueueHead-midiInQueueTail >= midiInQueueSize {
		return // очередь переполнена
	}
	idx := midiInQueueHead % midiInQueueSize
	for i := 0; i < n; i++ {
		midiInQueue[idx][i] = value[i]
	}
	midiInQueueLen[idx] = uint8(n)
	midiInQueueHead++
}

// midiInPop забирает следующий пакет из очереди.
func midiInPop() ([]byte, bool) {
	if midiInQueueTail == midiInQueueHead {
		return nil, false
	}
	idx := midiInQueueTail % midiInQueueSize
	n := int(midiInQueueLen[idx])
	midiInQueueTail++
	return midiInQueue[idx][:n], true
}

// handleBleMidiIn разбирает BLE MIDI пакет и пересылает MIDI на UART и USB (без эха в BLE).
func handleBleMidiIn(data []byte) {
	println("ble_midi_in: len=", len(data))
	if len(data) == 0 {
		return
	}
	if len(data) < 2 {
		println("ble_midi_in: packet too short")
		return
	}
	parseAndForwardBleMidi(data)
}

func parseAndForwardBleMidi(data []byte) {
	if len(data) < 2 {
		return
	}
	i := 1 // пропуск header (timestamp high)
	for i < len(data) {
		b := data[i]
		if b >= 0xF8 { // System Real-Time
			println("ble_midi_in: sys realtime=", b)
			forwardRawMidi(data[i : i+1])
			i++
			continue
		}
		if b&0x80 != 0 && b < 0xF0 {
			i++ // timestamp low
			if i >= len(data) {
				println("ble_midi_in: truncated after timestamp")
				return
			}
			b = data[i]
		}
		if b&0x80 == 0 {
			i++
			continue
		}
		status := b
		i++
		dlen := rawMidiDataLen(status)
		end := i + dlen
		if end > len(data) {
			end = len(data)
		}
		var msg [3]byte
		n := 1 + (end - i)
		msg[0] = status
		for j := 0; j < end-i; j++ {
			msg[1+j] = data[i+j]
		}
		if n > 1 {
			logBleMidiMessage(msg[:n])
			forwardRawMidi(msg[:n])
		}
		i = end
	}
}

func logBleMidiMessage(msg []byte) {
	switch msg[0] & 0xF0 {
	case 0x90:
		if len(msg) >= 3 {
			if msg[2] == 0 {
				println("ble_midi_in: Note Off ch=", msg[0]&0x0F, "note=", msg[1])
			} else {
				println("ble_midi_in: Note On ch=", msg[0]&0x0F, "note=", msg[1], "vel=", msg[2])
			}
		}
	case 0x80:
		if len(msg) >= 3 {
			println("ble_midi_in: Note Off ch=", msg[0]&0x0F, "note=", msg[1])
		}
	case 0xB0:
		if len(msg) >= 3 {
			println("ble_midi_in: CC ch=", msg[0]&0x0F, "cc=", msg[1], "val=", msg[2])
		}
	case 0xC0:
		if len(msg) >= 2 {
			println("ble_midi_in: PC ch=", msg[0]&0x0F, "prog=", msg[1])
		}
	default:
		println("ble_midi_in: status=", msg[0], "len=", len(msg))
	}
}

func rawMidiDataLen(status byte) int {
	switch status & 0xF0 {
	case 0xC0, 0xD0:
		return 1
	case 0x80, 0x90, 0xA0, 0xB0, 0xE0:
		return 2
	default:
		return 0
	}
}

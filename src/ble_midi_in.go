package main

const (
	midiInSlotSize  = 24
	midiInQueueSize = 256
)

// Очередь входящих BLE MIDI пакетов (WriteEvent → push, main loop → pop).
var midiInQueue [midiInQueueSize][midiInSlotSize]byte
var midiInQueueLen [midiInQueueSize]uint8
var midiInQueueHead uint16
var midiInQueueTail uint16

// midiInPush копирует пакет в очередь из контекста BLE WriteEvent (без аллокаций).
func midiInPush(value []byte) {
	n := len(value)
	if n > midiInSlotSize {
		n = midiInSlotSize
	}
	if midiInQueueHead-midiInQueueTail >= midiInQueueSize {
		return // очередь переполнена — пакет теряется
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
	if len(data) < 2 {
		return
	}
	parseAndForwardBleMidi(data)
}

// parseAndForwardBleMidi: timestamp → status → data для каждого сообщения в пакете.
func parseAndForwardBleMidi(data []byte) {
	if len(data) < 3 {
		return
	}
	i := 1 // пропуск header
	for i < len(data) {
		if data[i] >= 0xF8 {
			forwardRawMidi(data[i : i+1])
			i++
			continue
		}
		if data[i]&0x80 != 0 && data[i] < 0xF0 {
			i++
			if i >= len(data) {
				return
			}
		}
		if data[i]&0x80 == 0 {
			i++
			continue
		}
		status := data[i]
		i++
		dlen := rawMidiDataLen(status)
		if dlen == 0 || i+dlen > len(data) {
			return
		}
		var msg [3]byte
		msg[0] = status
		for j := 0; j < dlen; j++ {
			msg[1+j] = data[i+j]
		}
		forwardRawMidi(msg[:1+dlen])
		i += dlen
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

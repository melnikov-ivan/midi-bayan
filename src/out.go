// out.go — отправка MIDI через UART (DIN, 31250 бод), BLE MIDI и USB-MIDI.
//
// MIDI по DIN использует UART 31250 бод. На XIAO BLE:
//
//	TX = D6 (P1_11), RX = D7 (P1_12)
//
// BLE MIDI: стандартный сервис Apple/MIDI Bluetooth LE.
// Пакет: [header, timestamp, midi_status, data1, data2...]
//
// USB-MIDI: композитное устройство CDC+MIDI (TinyGo machine/usb/adc/midi).
// Каналы в прошивке 0–15; в пакете usb-midi — номера 1–16.
//
// Прошивка:
//
//	tinygo flash -target=xiao-ble .
package main

import (
	"machine"
	"machine/usb/adc/midi"
	"time"
)

const (
	midiBaud       = 31250
	usbMIDICableNr = 0 // один виртуальный кабель USB-MIDI
)

// bleMidiBuf — глобальный буфер для BLE MIDI пакетов (без аллокаций).
// Максимальный размер: 2 (header+ts) + 3 (MIDI CC) = 5 байт.
var bleMidiBuf [5]byte

// usbMIDIHostChannel переводит канал прошивки 0–15 в номер 1–16 для пакета machine/usb/adc/midi.
func usbMIDIHostChannel(channel uint8) uint8 {
	return (channel & 0x0F) + 1
}

// startMidiOut инициализирует UART для MIDI. Вызывается из controller перед отправкой нот.
func startMidiOut() {
	machine.UART0.Configure(machine.UARTConfig{
		BaudRate: midiBaud,
		TX:       machine.UART_TX_PIN, // D6, P1_11
		RX:       machine.UART_RX_PIN, // D7, P1_12
	})
	_ = midi.Port() // регистрация CDC+MIDI при линковке пакета midi (если init ещё не отработал)
}

// bleMidiHeader возвращает header и timestamp байты BLE MIDI пакета.
// Формат: header = 0x80 | (ms[12:7]), timestamp = 0x80 | (ms[6:0]),
// где ms — 13-битный счётчик миллисекунд с момента старта.
func bleMidiHeader() (header, ts byte) {
	ms := uint16(time.Now().UnixMilli() % 8192)
	return byte(0x80 | (ms >> 7)), byte(0x80 | (ms & 0x7F))
}

// sendMidiBLE оборачивает MIDI-сообщение в BLE MIDI пакет и отправляет через MidiChar.
// Ошибки игнорируются — BLE клиент может быть не подключён.
func sendMidiBLE(msg []byte) {
	n := len(msg)
	if n == 0 || n > 3 {
		return
	}
	bleMidiBuf[0], bleMidiBuf[1] = bleMidiHeader()
	for i := 0; i < n; i++ {
		bleMidiBuf[2+i] = msg[i]
	}
	MidiChar.Write(bleMidiBuf[:2+n]) //nolint:errcheck
}

// SendNoteOn отправляет MIDI Note On по UART, BLE MIDI и USB-MIDI (0x90 | channel, note, velocity).
func SendNoteOn(channel uint8, note, velocity uint8) {
	ch := channel & 0x0F
	msg := []byte{0x90 | ch, note & 0x7F, velocity & 0x7F}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
	_ = midi.Port().NoteOn(usbMIDICableNr, usbMIDIHostChannel(channel), midi.Note(note&0x7F), velocity&0x7F)
}

// SendNoteOff отправляет MIDI Note Off по UART, BLE MIDI и USB-MIDI (0x80 | channel, note, 0).
func SendNoteOff(channel uint8, note uint8) {
	ch := channel & 0x0F
	msg := []byte{0x80 | ch, note & 0x7F, 0}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
	_ = midi.Port().NoteOff(usbMIDICableNr, usbMIDIHostChannel(channel), midi.Note(note&0x7F), 0)
}

// SendProgramChange отправляет MIDI Program Change по UART, BLE MIDI и USB-MIDI (0xC0 | channel, program).
func SendProgramChange(channel uint8, program uint8) {
	ch := channel & 0x0F
	msg := []byte{0xC0 | ch, program & 0x7F}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
	_ = midi.Port().ProgramChange(usbMIDICableNr, usbMIDIHostChannel(channel), program&0x7F)
}

// SendVolume отправляет MIDI Control Change #7 (Channel Volume) по UART, BLE MIDI и USB-MIDI (0xB0 | channel, 0x07, value).
func SendVolume(channel uint8, volume uint8) {
	sendCC(channel, 0x07, volume)
}

// SendReverb отправляет MIDI Control Change #91 (Effects 1 Depth — Reverb Send).
func SendReverb(channel uint8, value uint8) {
	sendCC(channel, 91, value)
}

// SendChorus отправляет MIDI Control Change #93 (Effects 3 Depth — Chorus Send).
func SendChorus(channel uint8, value uint8) {
	sendCC(channel, 93, value)
}

// SendDelay отправляет MIDI Control Change #94 (Effects 4 Depth — используется как Delay).
func SendDelay(channel uint8, value uint8) {
	sendCC(channel, 94, value)
}

// SendAllNotesOff отправляет MIDI CC#123 (All Notes Off) на все 16 каналов.
func SendAllNotesOff() {
	for ch := uint8(0); ch < 16; ch++ {
		sendCC(ch, 123, 0)
	}
}

// sendCC формирует и отправляет MIDI Control Change по UART, BLE MIDI и USB-MIDI.
func sendCC(channel, controller, value uint8) {
	ch := channel & 0x0F
	msg := []byte{0xB0 | ch, controller & 0x7F, value & 0x7F}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
	_ = midi.Port().ControlChange(usbMIDICableNr, usbMIDIHostChannel(channel), controller&0x7F, value&0x7F)
}

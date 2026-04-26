// out.go — отправка MIDI через UART (DIN, 31250 бод) и BLE MIDI.
//
// MIDI по DIN использует UART 31250 бод. На XIAO BLE:
//   TX = D6 (P1_11), RX = D7 (P1_12)
//
// BLE MIDI: стандартный сервис Apple/MIDI Bluetooth LE.
// Пакет: [header, timestamp, midi_status, data1, data2...]
//
// Прошивка:
//   tinygo flash -target=xiao-ble .
package main

import (
	"machine"
	"time"
)

const (
	midiBaud = 31250
)

// bleMidiBuf — глобальный буфер для BLE MIDI пакетов (без аллокаций).
// Максимальный размер: 2 (header+ts) + 3 (MIDI CC) = 5 байт.
var bleMidiBuf [5]byte

// startMidiOut инициализирует UART для MIDI. Вызывается из controller перед отправкой нот.
func startMidiOut() {
	machine.UART0.Configure(machine.UARTConfig{
		BaudRate: midiBaud,
		TX:       machine.UART_TX_PIN, // D6, P1_11
		RX:       machine.UART_RX_PIN, // D7, P1_12
	})
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

// SendNoteOn отправляет MIDI Note On по UART и BLE MIDI (0x90 | channel, note, velocity).
func SendNoteOn(channel uint8, note, velocity uint8) {
	ch := channel & 0x0F
	msg := []byte{0x90 | ch, note & 0x7F, velocity & 0x7F}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
}

// SendNoteOff отправляет MIDI Note Off по UART и BLE MIDI (0x80 | channel, note, 0).
func SendNoteOff(channel uint8, note uint8) {
	ch := channel & 0x0F
	msg := []byte{0x80 | ch, note & 0x7F, 0}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
}

// SendProgramChange отправляет MIDI Program Change по UART и BLE MIDI (0xC0 | channel, program).
func SendProgramChange(channel uint8, program uint8) {
	ch := channel & 0x0F
	msg := []byte{0xC0 | ch, program & 0x7F}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
}

// SendVolume отправляет MIDI Control Change #7 (Channel Volume) по UART и BLE MIDI (0xB0 | channel, 0x07, value).
func SendVolume(channel uint8, volume uint8) {
	ch := channel & 0x0F
	msg := []byte{0xB0 | ch, 0x07, volume & 0x7F}
	machine.UART0.Write(msg)
	sendMidiBLE(msg)
}

package main

import (
	"time"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

// Буфер значения характеристики — глобальный, без аллокаций в прерывании (при чтении/записи BLE).
var charValueBuf [64]byte
var charValueLen int = 1 // начальное значение: 1 байт (0)
var hasNewValue bool     // флаг: в WriteEvent записали новое значение (вывод только по нему)

// MidiChar — характеристика стандартного BLE MIDI сервиса.
// Используется в out.go для отправки MIDI-сообщений через BLE.
var MidiChar bluetooth.Characteristic

// StartBLEService включает адаптер, регистрирует сервисы, запускает рекламу и блокируется.
// Вызывать из main в отдельной горутине: go StartBLEService().
func StartBLEService() {
	must(adapter.Enable())

	// --- Стандартный BLE MIDI сервис ---
	// Service UUID: 03B80E5A-EDE8-4B33-A751-6CE34EC4C700
	// Characteristic UUID: 7772E5DB-3868-4112-A1A9-F2669D106BF3
	midiServiceUUID := bluetooth.NewUUID([16]byte{
		0x03, 0xB8, 0x0E, 0x5A, 0xED, 0xE8, 0x4B, 0x33,
		0xA7, 0x51, 0x6C, 0xE3, 0x4E, 0xC4, 0xC7, 0x00,
	})
	midiCharUUID := bluetooth.NewUUID([16]byte{
		0x77, 0x72, 0xE5, 0xDB, 0x38, 0x68, 0x41, 0x12,
		0xA1, 0xA9, 0xF2, 0x66, 0x9D, 0x10, 0x6B, 0xF3,
	})

	must(adapter.AddService(&bluetooth.Service{
		UUID: midiServiceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &MidiChar,
				UUID:   midiCharUUID,
				// Write нужен для регистрации WriteEvent в TinyGo/nRF (gatts_sd.go:
				// handler вешается только при CharacteristicWritePermission).
				// WriteWithoutResponse — основной путь от PWA (writeValueWithoutResponse).
				Flags: bluetooth.CharacteristicReadPermission |
					bluetooth.CharacteristicWriteWithoutResponsePermission |
					bluetooth.CharacteristicWritePermission |
					bluetooth.CharacteristicNotifyPermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					midiInPush(value)
				},
			},
		},
	}))

	// --- Config сервис (get/set program) ---
	// 128-bit UUID'ы (произвольные)
	configServiceUUID := bluetooth.NewUUID([16]byte{
		0x12, 0x34, 0x56, 0x78,
		0x12, 0x34,
		0x56, 0x78,
		0x12, 0x34,
		0x56, 0x78, 0x90, 0xab, 0xcd, 0xef,
	})

	configCharUUID := bluetooth.NewUUID([16]byte{
		0xfe, 0xdc, 0xba, 0x09,
		0x87, 0x65,
		0x43, 0x21,
		0x87, 0x65,
		0x43, 0x21, 0x10, 0x32, 0x54, 0x76,
	})

	// Переменная для хранения характеристики
	var configChar bluetooth.Characteristic

	// Начальное значение — срез глобального буфера, без аллокации при чтении клиентом
	charValueBuf[0] = 0

	must(adapter.AddService(&bluetooth.Service{
		UUID: configServiceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			{
				Handle: &configChar,
				UUID:   configCharUUID,
				Value:  charValueBuf[:charValueLen],
				Flags:  bluetooth.CharacteristicReadPermission | bluetooth.CharacteristicWritePermission | bluetooth.CharacteristicNotifyPermission,
				WriteEvent: func(client bluetooth.Connection, offset int, value []byte) {
					// В контексте прерывания нельзя делать аллокации (string, append, char.Write и т.д.).
					// Только копируем в глобальный буфер и ставим флаг.
					n := len(value)
					if n > len(charValueBuf) {
						n = len(charValueBuf)
					}
					for i := 0; i < n; i++ {
						charValueBuf[i] = value[i]
					}
					charValueLen = n
					hasNewValue = true
				},
			},
		},
	}))

	// Реклама: пакет ограничен 31 байтом.
	// Flags(3) + Name("Bayan"=7) + 128-bit UUID(18) = 28 байт — влезает.
	// Полный 128-bit UUID MIDI-сервиса нужен macOS CoreMIDI для обнаружения.
	adv := adapter.DefaultAdvertisement()
	must(adv.Configure(bluetooth.AdvertisementOptions{
		LocalName:    "Bayan",
		ServiceUUIDs: []bluetooth.UUID{midiServiceUUID},
	}))
	must(adv.Start())

	println("BLE peripheral started, advertising as 'Midi-Bayan'")

	// Бесконечный цикл: парсим входящие сообщения и синхронизируем только при новом значении
	for {
		busy := false
		for {
			data, ok := midiInPop()
			if !ok {
				break
			}
			handleBleMidiIn(data)
			busy = true
		}
		if hasNewValue && charValueLen > 0 {
			hasNewValue = false
			msg := charValueBuf[:charValueLen]
			cmd, payload, ok := parseMessage(msg)
			if !ok {
				println("parse error or bad CRC, len=", charValueLen)
				continue
			}
			switch cmd {
			case cmdGetProgram:
				if ch, inst, vol, oct, ok := handleGetProgram(payload); ok {
					charValueBuf[0] = cmdGetProgram
					charValueBuf[1] = ch
					charValueBuf[2] = inst
					charValueBuf[3] = vol
					charValueBuf[4] = oct
					configChar.Write(charValueBuf[:5])
				}
			case cmdSetProgram:
				handleSetProgram(payload)
			case cmdGetAudio:
				if rev, chor, del, ok := handleGetAudio(payload); ok {
					charValueBuf[0] = cmdGetAudio
					charValueBuf[1] = rev
					charValueBuf[2] = chor
					charValueBuf[3] = del
					configChar.Write(charValueBuf[:4])
				}
			case cmdSetAudio:
				handleSetAudio(payload)
			case cmdStyle:
				handleStyle(payload)
			case cmdRecord:
				handleRecord(payload)
			case cmdTempo:
				if bpm, ok := handleTempo(payload); ok {
					charValueBuf[0] = cmdTempo
					charValueBuf[1] = byte(bpm)
					charValueBuf[2] = byte(bpm >> 8)
					charValueBuf[3] = 0
					charValueBuf[4] = 0
					configChar.Write(charValueBuf[:5])
				}
			default:
				println("unknown command:", cmd)
			}
		}
		if !busy && !hasNewValue {
			time.Sleep(2 * time.Millisecond)
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

package main

import (
	"time"
)

const (
	cmdGetProgram byte = 0x01
	cmdSetProgram byte = 0x02
	cmdSetAudio   byte = 0x03
	cmdGetAudio   byte = 0x04
	cmdStyle      byte = 0x05 // стиль / пуск (PWA: экран «Стиль»)
	cmdTempo      byte = 0x06 // тап по «Темп» в PWA + ответ BPM по BLE
)

// audioBroadcastChannels — каналы, на которые транслируются общие аудио-настройки.
// 0 — Melody, 1 — Chord, 2 — Bass (см. pwa/index.html).
var audioBroadcastChannels = [...]byte{0, 1, 2}

const minMessageLen = 4 // cmd(1) + len(2) + crc(1), payload может быть 0

// crc8 считает CRC-8 (полином 0x07) по данным без последнего байта (место CRC).
func crc8(data []byte) byte {
	var crc byte = 0
	for _, b := range data {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// parseMessage разбирает буфер: 1 байт команда, 2 байта длина payload (little-endian), payload, 1 байт CRC.
// Возвращает команду, payload и true при успехе; при ошибке ok == false.
func parseMessage(buf []byte) (cmd byte, payload []byte, ok bool) {
	if len(buf) < minMessageLen {
		return 0, nil, false
	}
	cmd = buf[0]
	payloadLen := int(buf[1]) | int(buf[2])<<8
	totalLen := 1 + 2 + payloadLen + 1
	if len(buf) < totalLen {
		return 0, nil, false
	}
	payload = buf[3 : 3+payloadLen]
	dataWithCrc := buf[:totalLen]
	gotCrc := dataWithCrc[totalLen-1]
	expectedCrc := crc8(dataWithCrc[:totalLen-1])
	if gotCrc != expectedCrc {
		return 0, nil, false
	}
	return cmd, payload, true
}

// handleGetProgram обрабатывает команду get_program: payload = [channel, ...].
// Instrument, Volume и Octave для ответа берутся из config по указанному channel.
// Возвращает channel, instrument, volume, octave и true при успехе; иначе ok == false.
func handleGetProgram(payload []byte) (channel, instrument, volume, octave byte, ok bool) {
	if len(payload) != 3 {
		return 0, 0, 0, 0, false
	}
	channel = payload[0]
	instrument, volume, octave = GetChannelConfig(channel)
	println("get_program: channel=", channel, "instrument=", instrument, "volume=", volume, "octave=", octave)
	return channel, instrument, volume, octave, true
}

// handleSetProgram обрабатывает команду set_program: payload = [channel, instrument, volume, octave].
// Сохраняет настройки в ChannelConfigs и отправляет Event (Program Change) в EventChannel, если канал задан.
func handleSetProgram(payload []byte) bool {
	if len(payload) != 4 {
		return false
	}
	channel := payload[0]
	instrument := payload[1]
	volume := payload[2]
	octave := payload[3]
	SetChannelConfig(channel, instrument, volume, octave)
	println("set_program: channel=", channel, "instrument=", instrument, "volume=", volume, "octave=", octave)
	if EventChannel != nil {
		EventChannel <- Event{
			Type:    ProgramChange,
			Channel: channel,
			Program: instrument,
		}
		EventChannel <- Event{
			Type:    Volume,
			Channel: channel,
			Volume:  volume,
		}
	}
	return true
}

// handleGetAudio обрабатывает команду get_audio: payload пуст.
// Возвращает текущие общие аудио-настройки (volume, reverb, chorus, delay).
func handleGetAudio(payload []byte) (volume, reverb, chorus, delay byte, ok bool) {
	if len(payload) != 0 {
		return 0, 0, 0, 0, false
	}
	a := AudioConfig
	println("get_audio: volume=", a.Volume, "reverb=", a.Reverb, "chorus=", a.Chorus, "delay=", a.Delay)
	return a.Volume, a.Reverb, a.Chorus, a.Delay, true
}

// handleSetAudio обрабатывает команду set_audio: payload = [volume, reverb, chorus, delay].
// Сохраняет общие аудио-настройки и рассылает соответствующие MIDI CC через EventChannel
// на все используемые каналы (audioBroadcastChannels).
func handleSetAudio(payload []byte) bool {
	if len(payload) != 4 {
		return false
	}
	volume := payload[0]
	reverb := payload[1]
	chorus := payload[2]
	delay := payload[3]
	SetAudioConfig(volume, reverb, chorus, delay)
	println("set_audio: volume=", volume, "reverb=", reverb, "chorus=", chorus, "delay=", delay)
	if EventChannel != nil {
		for _, ch := range audioBroadcastChannels {
			EventChannel <- Event{Type: Volume, Channel: ch, Volume: volume}
			EventChannel <- Event{Type: Reverb, Channel: ch, CCValue: reverb}
			EventChannel <- Event{Type: Chorus, Channel: ch, CCValue: chorus}
			EventChannel <- Event{Type: Delay, Channel: ch, CCValue: delay}
		}
	}
	return true
}

// handleStyle: payload пуст — пуск; payload [1..4] — сохранить стиль (pop/rock/disco/waltz).
func handleStyle(payload []byte) bool {
	if len(payload) == 0 {
		println("style_play, style=", SelectedStyle())
		return true
	}
	if len(payload) != 1 {
		return false
	}
	style := payload[0]
	if style < 1 || style > 4 {
		return false
	}
	SetSelectedStyle(style)
	println("style_set:", style)
	return true
}

// handleTempo: каждое нажатие «Темп» в PWA — тап; BPM по интервалу между двумя последними тапами.
func handleTempo(payload []byte) (bpm uint16, ok bool) {
	if len(payload) != 0 {
		return 0, false
	}
	bpm = TapTempo(time.Now().UnixMilli())
	println("tempo: bpm=", bpm, "interval_ms=", TempoBeatIntervalMs())
	return bpm, true
}

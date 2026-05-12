package main

// Config хранит настройки канала: номер канала, инструмент, громкость и октаву.
type Config struct {
	Channel    byte
	Instrument byte
	Volume     byte
	Octave     byte
}

// Конфиги по каналам (0–15). Индекс = номер канала.
var ChannelConfigs [16]Config

func init() {
	for i := 0; i < 16; i++ {
		ChannelConfigs[i] = Config{
			Channel:    byte(i),
			Instrument: byte(i), // Acoustic Grand Piano
			Volume:     100,     // громкость по умолчанию (0–127)
			Octave:     4,       // средняя октава
		}
	}
}

// GetChannelConfig возвращает Instrument, Volume и Octave для канала channel.
// Если channel >= 16, возвращает 0, 0, 0.
func GetChannelConfig(channel byte) (instrument, volume, octave byte) {
	if channel >= 16 {
		return 0, 0, 0
	}
	c := ChannelConfigs[channel]
	return c.Instrument, c.Volume, c.Octave
}

// SetChannelConfig сохраняет instrument, volume и octave для канала channel в ChannelConfigs.
// Если channel >= 16, ничего не делает.
func SetChannelConfig(channel, instrument, volume, octave byte) {
	if channel >= 16 {
		return
	}
	ChannelConfigs[channel] = Config{
		Channel:    channel,
		Instrument: instrument,
		Volume:     volume,
		Octave:     octave,
	}
}

// AudioSettings — общие аудио-параметры: громкость и глубина эффектов (0–127).
type AudioSettings struct {
	Volume byte
	Reverb byte
	Chorus byte
	Delay  byte
}

// AudioConfig — текущие общие аудио-настройки, применяемые ко всем используемым каналам.
var AudioConfig = AudioSettings{
	Volume: 100,
	Reverb: 0,
	Chorus: 0,
	Delay:  0,
}

// SetAudioConfig сохраняет общие аудио-настройки.
func SetAudioConfig(volume, reverb, chorus, delay byte) {
	AudioConfig = AudioSettings{
		Volume: volume,
		Reverb: reverb,
		Chorus: chorus,
		Delay:  delay,
	}
}

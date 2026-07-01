package main

// recording — включена ли запись MIDI на стороне PWA (сама запись событий происходит в браузере,
// прошивка лишь отслеживает состояние).
var recording bool

// ToggleRecording переключает состояние записи и возвращает новое значение.
func ToggleRecording() bool {
	recording = !recording
	return recording
}

// IsRecording возвращает текущее состояние записи.
func IsRecording() bool {
	return recording
}

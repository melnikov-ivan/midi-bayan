package main

// Интервал между последними двумя нажатиями «Темп» (CMD_TEMPO), мс; 0 — ещё не было пары валидных тапов.
var tempoBeatIntervalMs int64

// Время последнего CMD_TEMPO (Unix мс).
var tempoLastTapMs int64

// selectedStyle — выбранный стиль (1=pop, 2=rock, 3=disco, 4=waltz).
var selectedStyle byte = 1

func SelectedStyle() byte {
	return selectedStyle
}

func SetSelectedStyle(style byte) {
	selectedStyle = style
}

// TapTempo регистрирует тап в nowMs и возвращает оценку BPM.
func TapTempo(nowMs int64) uint16 {
	if tempoLastTapMs > 0 {
		d := nowMs - tempoLastTapMs
		// ~20–300 BPM
		if d >= 180 && d <= 3000 {
			tempoBeatIntervalMs = d
		}
	}
	tempoLastTapMs = nowMs

	if tempoBeatIntervalMs <= 0 {
		return 120
	}
	x := (60000 + tempoBeatIntervalMs/2) / tempoBeatIntervalMs
	if x < 20 {
		x = 20
	}
	if x > 320 {
		x = 320
	}
	return uint16(x)
}

func TempoBeatIntervalMs() int64 {
	return tempoBeatIntervalMs
}

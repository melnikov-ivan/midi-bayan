package main

import "time"

const (
	drumChannel    = 9 // GM percussion (channel 10)
	noteKick       = 36
	noteSnare      = 38
	noteHiHat      = 42
	styleMetronome = 0
	stylePop       = 1
	styleRock      = 2
	styleDisco     = 3
	styleWaltz     = 4
)

// Интервал между последними двумя нажатиями «Темп» (CMD_TEMPO), мс; 0 — ещё не было пары валидных тапов.
var tempoBeatIntervalMs int64

// Время последнего CMD_TEMPO (Unix мс).
var tempoLastTapMs int64

// selectedStyle — выбранный стиль (0=metronome, 1=pop, 2=rock, 3=disco, 4=waltz).
var selectedStyle byte = styleMetronome

var playing bool

type drumStep struct {
	kick, snare, hat bool
}

// metronomePattern — щелчок на каждую четверть; акцент (kick) на первой доле.
var metronomePattern = []drumStep{
	{kick: true},
	{},
	{hat: true},
	{},
	{hat: true},
	{},
	{hat: true},
	{},
}

// popPattern — 8 долей в такте: kick на 1 и 3, snare на 2 и 4, hi-hat на каждую долю.
var popPattern = []drumStep{
	{kick: true, hat: true},
	{hat: true},
	{snare: true, hat: true},
	{hat: true},
	{kick: true, hat: true},
	{hat: true},
	{snare: true, hat: true},
	{hat: true},
}

// rockPattern — прямой рок-бит с более плотной бочкой: kick на 1, 2&, 3; snare на 2 и 4; hi-hat на каждую восьмую.
var rockPattern = []drumStep{
	{kick: true, hat: true},
	{hat: true},
	{snare: true, hat: true},
	{kick: true, hat: true},
	{kick: true, hat: true},
	{hat: true},
	{snare: true, hat: true},
	{hat: true},
}

// discoPattern — «four on the floor»: kick на каждую четверть, snare на 2 и 4, hi-hat на каждую восьмую.
var discoPattern = []drumStep{
	{kick: true, hat: true},
	{hat: true},
	{kick: true, snare: true, hat: true},
	{hat: true},
	{kick: true, hat: true},
	{hat: true},
	{kick: true, snare: true, hat: true},
	{hat: true},
}

// waltzPattern — размер 3/4: kick на 1, snare на 2 и 3, hi-hat на каждую восьмую.
var waltzPattern = []drumStep{
	{kick: true, hat: true},
	{hat: true},
	{snare: true, hat: true},
	{hat: true},
	{snare: true, hat: true},
	{hat: true},
}

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

// play переключает воспроизведение: первый вызов — старт, повторный — стоп.
func play() {
	if playing {
		stop()
		return
	}
	playing = true
	go runPlayerLoop()
}

func stop() {
	playing = false
	SendNoteOff(drumChannel, noteKick)
	SendNoteOff(drumChannel, noteSnare)
	SendNoteOff(drumChannel, noteHiHat)
	println("style_stop")
}

// PlayEmbeddedMIDI parses and plays the embedded test MIDI file once.
func PlayEmbeddedMIDI() error {
	mf, err := ReadTestMIDI()
	if err != nil {
		return err
	}
	playMIDIFile(mf, 120)
	return nil
}

func playMIDIFile(mf *MIDIFile, bpm uint16) {
	if mf == nil || mf.Division == 0 {
		return
	}
	for _, ev := range mf.Events {
		sleepTicks(ev.DeltaTicks, mf.Division, bpm)
		switch ev.Type {
		case ReaderEventNoteOn:
			if ev.Data2 == 0 {
				SendNoteOff(ev.Channel, ev.Data1)
			} else {
				SendNoteOn(ev.Channel, ev.Data1, ev.Data2)
			}
		case ReaderEventNoteOff:
			SendNoteOff(ev.Channel, ev.Data1)
		case ReaderEventProgramChange:
			SendProgramChange(ev.Channel, ev.Data1)
		case ReaderEventControlChange:
			sendCC(ev.Channel, ev.Data1, ev.Data2)
		}
	}
}

func sleepTicks(delta uint32, division uint16, bpm uint16) {
	if delta == 0 || division == 0 {
		return
	}
	if bpm == 0 {
		bpm = 120
	}
	usPerQuarter := uint32(60000000 / uint32(bpm))
	us := (uint64(delta) * uint64(usPerQuarter)) / uint64(division)
	if us == 0 {
		return
	}
	time.Sleep(time.Duration(us) * time.Microsecond)
}

func runPlayerLoop() {
	for playing {
		pattern := patternForStyle(SelectedStyle())
		stepDur := eighthNoteDuration()
		for _, step := range pattern {
			if !playing {
				return
			}
			playDrumStep(step)
			time.Sleep(stepDur)
		}
	}
}

func patternForStyle(style byte) []drumStep {
	switch style {
	case styleMetronome:
		return metronomePattern
	case stylePop:
		return popPattern
	case styleRock:
		return rockPattern
	case styleDisco:
		return discoPattern
	case styleWaltz:
		return waltzPattern
	default:
		return metronomePattern
	}
}

func eighthNoteDuration() time.Duration {
	interval := TempoBeatIntervalMs()
	if interval <= 0 {
		return 250 * time.Millisecond // 120 BPM, восьмая ≈ 250 мс
	}
	return time.Duration(interval/2) * time.Millisecond
}

func playDrumStep(s drumStep) {
	const vel = 100
	if s.kick {
		SendNoteOn(drumChannel, noteKick, vel)
	}
	if s.snare {
		SendNoteOn(drumChannel, noteSnare, vel)
	}
	if s.hat {
		SendNoteOn(drumChannel, noteHiHat, vel)
	}
	time.Sleep(35 * time.Millisecond)
	if s.kick {
		SendNoteOff(drumChannel, noteKick)
	}
	if s.snare {
		SendNoteOff(drumChannel, noteSnare)
	}
	if s.hat {
		SendNoteOff(drumChannel, noteHiHat)
	}
}

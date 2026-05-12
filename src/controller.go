package main

// MIDI-команды отправляются через UART (DIN, 31250 бод). См. out.go.
//
//   tinygo flash -target=xiao-ble .
//   tinygo monitor
import (
	"machine"
	"time"
)

// KeyEventType — тип события: нота (клавиша) или смена программы.
type KeyEventType uint8

const (
	NoteOn        KeyEventType = iota // событие клавиши (Channel, Note, Velocity: 100=нажато, 0=отпущено)
	ProgramChange                     // смена инструмента (Channel, Program)
	Volume                            // громкость канала (Channel, Volume)
)

type Event struct {
	Type KeyEventType // NoteOn, ProgramChange или Volume

	// NoteOn: клавиатура заполняет из keymap (Velocity: 100=нажато, 0=отпущено)
	Channel  uint8
	Note     uint8
	Velocity uint8

	// ProgramChange
	Program uint8

	// Volume (CC #7)
	Volume uint8
}

var led = machine.LED

// EventChannel — общий канал событий: клавиатура (RunKeyboard) и BLE (handleSetProgram Program Change).
var EventChannel chan Event

func main() {
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led.Low()

	startMidiOut()
	println("MIDI Controller (UART) запущен")

	EventChannel = make(chan Event, 8)
	go StartBLEService()
	go RunKeyboard(EventChannel)

	// Ноты и параметры MIDI берутся из keymap; Program Change приходит из BLE (handleSetProgram).
	for ev := range EventChannel {
		switch ev.Type {
		case ProgramChange:
			SendProgramChange(ev.Channel, ev.Program)
			println("MIDI: Program Change ch=", ev.Channel, "program=", ev.Program)
			blink()
		case Volume:
			SendVolume(ev.Channel, ev.Volume)
			println("MIDI: Volume ch=", ev.Channel, "volume=", ev.Volume)
			blink()
		case NoteOn:
			SendNoteOn(ev.Channel, ev.Note, ev.Velocity)
			if ev.Velocity > 0 {
				println("MIDI: Note On ", ev.Note)
			} else {
				println("MIDI: Note Off", ev.Note)
			}
			blink()
		default:
			// неизвестный тип — пропускаем
		}
	}
}

func blink() {
	led.High()
	time.Sleep(50 * time.Millisecond)
	led.Low()
}

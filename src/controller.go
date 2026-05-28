package main

// MIDI-команды: UART (DIN), BLE MIDI, USB-MIDI. См. out.go.
//
//   tinygo flash -target=xiao-ble .
//   tinygo monitor
import (
	"machine"
	"sync"
	"time"
)

// KeyEventType — тип события: нота (клавиша) или смена программы.
type KeyEventType uint8

const (
	NoteOn        KeyEventType = iota // событие клавиши (Channel, Note, Velocity: 100=нажато)
	NoteOff                           // отпускание ноты (Channel, Note)
	ProgramChange                     // смена инструмента (Channel, Program)
	Volume                            // громкость канала (Channel, Volume)
	Reverb                            // глубина реверберации канала (Channel, CCValue)
	Chorus                            // глубина хоруса канала (Channel, CCValue)
	Delay                             // глубина дилея канала (Channel, CCValue)
)

type Event struct {
	Type KeyEventType // NoteOn, NoteOff, ProgramChange, Volume, Reverb, Chorus, Delay

	// NoteOn: клавиатура заполняет из keymap (Velocity: 100=нажато; 0=отпущено — тоже NoteOn)
	Channel  uint8
	Note     uint8
	Velocity uint8

	// ProgramChange
	Program uint8

	// Volume (CC #7)
	Volume uint8

	// Reverb (CC #91), Chorus (CC #93), Delay (CC #94) — общее поле значения
	CCValue uint8
}

var led = machine.LED

// EventChannel — интерактивные события: клавиатура и BLE (конфиг). SMF-плеер шлёт MIDI через dispatchEvent.
var EventChannel chan Event

var midiOutMu sync.Mutex

// dispatchEvent отправляет MIDI на все выходы; безопасен при вызове из нескольких горутин (SMF format 1).
func dispatchEvent(ev Event) {
	midiOutMu.Lock()
	defer midiOutMu.Unlock()
	switch ev.Type {
	case ProgramChange:
		SendProgramChange(ev.Channel, ev.Program)
	case Volume:
		SendVolume(ev.Channel, ev.Volume)
	case Reverb:
		SendReverb(ev.Channel, ev.CCValue)
	case Chorus:
		SendChorus(ev.Channel, ev.CCValue)
	case Delay:
		SendDelay(ev.Channel, ev.CCValue)
	case NoteOff:
		SendNoteOff(ev.Channel, ev.Note)
	case NoteOn:
		if ev.Velocity == 0 {
			SendNoteOff(ev.Channel, ev.Note)
		} else {
			SendNoteOn(ev.Channel, ev.Note, ev.Velocity)
		}
	}
}

func main() {
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led.Low()

	startMidiOut()
	println("MIDI Controller (UART) запущен")

	EventChannel = make(chan Event, 32)
	go StartBLEService()
	go RunKeyboard(EventChannel)

	// go demo()

	// Ноты и параметры MIDI берутся из keymap; Program Change приходит из BLE (handleSetProgram).
	for ev := range EventChannel {
		dispatchEvent(ev)
		switch ev.Type {
		case ProgramChange:
			println("MIDI: Program Change ch=", ev.Channel, "program=", ev.Program)
		case Volume:
			println("MIDI: Volume ch=", ev.Channel, "volume=", ev.Volume)
		case Reverb:
			println("MIDI: Reverb ch=", ev.Channel, "value=", ev.CCValue)
		case Chorus:
			println("MIDI: Chorus ch=", ev.Channel, "value=", ev.CCValue)
		case Delay:
			println("MIDI: Delay ch=", ev.Channel, "value=", ev.CCValue)
		case NoteOff:
			println("MIDI: Note Off", ev.Note)
		case NoteOn:
			if ev.Velocity == 0 {
				println("MIDI: Note Off", ev.Note)
			} else {
				println("MIDI: Note On ", ev.Note)
			}
		default:
			continue
		}
		blink()
	}
}

func blink() {
	led.High()
	// time.Sleep(50 * time.Millisecond)
	led.Low()
}

// demo периодически играет ноту C4: 100 мс звук, затем пауза 2 с до следующего срабатывания.
// Запускается из main параллельно циклу событий, чтобы не блокировать приём с клавиатуры/BLE.
func demo() {
	const demoNote = 60 // C4
	ch := uint8(DefaultChannel)
	for {
		SendNoteOn(ch, demoNote, DefaultVelocity)
		time.Sleep(500 * time.Millisecond)
		SendNoteOff(ch, demoNote)
		println("demo: note", demoNote, "100ms, next in 2s")
		time.Sleep(2 * time.Second)
	}
}

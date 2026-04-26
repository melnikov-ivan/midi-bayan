package main

// tinygo flash -target=waveshare-rp2040-zero
// tinygo flash -target=xiao-ble keyboard.go
// tinygo monitor
import (
	"machine"
	"time"
)

var (

	// Пины для подключения 74HC165N
	// SH/LD (Shift/Load) - управляющий пин для загрузки параллельных данных
	shiftLoadPin = machine.D0

	// CLK (Clock) - тактовый сигнал для сдвига данных
	clockPin = machine.D1

	// QH (Serial Output) - последовательный выход данных
	dataPin = machine.D2
)

// readShiftRegister читает totalBits данных из цепочки 74HC165N.
// 74HC165 сдвигает по фронту CLK (LOW→HIGH). После сдвига нужно дать время
// выходу QH установиться (tpd ~20–30 ns), иначе читается старый/неверный бит.
const shiftDelay = 1 * time.Microsecond
const (
	bitsPerBoard        = 8
	shiftRegisterBoards = 5
	totalBits           = bitsPerBoard * shiftRegisterBoards
)

func readShiftRegister() uint64 {
	var data uint64 = 0

	// Шаг 1: Загрузка параллельных данных (SH/LD = LOW)
	shiftLoadPin.Low()
	time.Sleep(shiftDelay)

	// Шаг 2: Режим сдвига (SH/LD = HIGH) — первый бит уже на QH
	shiftLoadPin.High()
	time.Sleep(shiftDelay)

	// Шаг 3: Читаем totalBits бит. Порядок: прочитать QH → такт → (новый бит на QH).
	// Первый бит уже на QH после load; складываем в data[0]..data[totalBits-1] по мере сдвига.
	for i := 0; i < totalBits; i++ {
		if i > 0 {
			// Сдвиг: фронт LOW→HIGH. После сдвига на QH появляется следующий бит.
			clockPin.High()
			time.Sleep(shiftDelay) // время установки QH после сдвига
			clockPin.Low()
			time.Sleep(shiftDelay)
		}
		// Читаем бит с QH (первый сдвинутый бит — в младший бит data, дальше к старшим).
		if dataPin.Get() {
			data |= (1 << uint(i))
		}
	}

	return data
}

// RunKeyboard читает регистр сдвига, при изменении бита отправляет Event в канал ch.
// Закрывать ch не нужно — цикл бесконечный.
func RunKeyboard(ch chan<- Event) {
	// Настройка пинов для 74HC165N
	shiftLoadPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	shiftLoadPin.High()
	clockPin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	clockPin.Low()
	dataPin.Configure(machine.PinConfig{Mode: machine.PinInput})

	var prev uint64 = 0
	for {
		data := readShiftRegister()
		for i := 0; i < totalBits; i++ {
			mask := uint64(1 << uint(i))
			was := (prev & mask) != 0
			now := (data & mask) != 0
			if was != now {
				if i < len(BitToNote) {
					Velocity := uint8(0)
					if now {
						Velocity = DefaultVelocity
					}
					ch <- Event{
						Type:     NoteOn,
						Channel:  DefaultChannel,
						Note:     BitToNote[i],
						Velocity: Velocity,
					}
				}
			}
		}
		prev = data
		time.Sleep(1 * time.Millisecond)
	}
}

// formatBinary форматирует байт в бинарную строку для удобного отображения
func formatBinary(b uint8) string {
	var result string
	for i := 7; i >= 0; i-- {
		if b&(1<<i) != 0 {
			result += "1"
		} else {
			result += "0"
		}
	}
	return result
}

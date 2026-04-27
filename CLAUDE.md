# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TinyGo firmware for a MIDI Bayan (electronic accordion) controller running on a XIAO BLE (nRF52840). Reads 40 keys via 5 chained 74HC165 shift registers, sends MIDI over both UART (DIN 31250 baud) and Bluetooth LE. A companion PWA in `pwa/` provides wireless configuration over BLE.

## Build & Flash Commands

```sh
# Flash firmware to device (must be in bootloader/UF2 mode)
make flash
# or equivalently:
tinygo flash -target=xiao-ble .

# Build UF2 without flashing
tinygo build -target=xiao-ble -o firmware.uf2 .

# Monitor serial debug output
make monitor
# or:
tinygo monitor
```

There are no tests or linters configured.

## Architecture

The firmware uses a single event channel to decouple input from output:

```
keyboard.go  →  chan Event  →  controller.go  →  out.go (UART + BLE MIDI)
                                                ↕
                              server.go (BLE GATT config service)
                                                ↕
                              api.go / config.go (protocol + state)
```

**Key files:**
- `controller.go` — `main()`: initializes hardware, starts `StartBLEService()` and `RunKeyboard()` goroutines, then loops on the event channel dispatching MIDI output
- `keyboard.go` — polls 5×74HC165 shift registers via SPI-like bit-bang (SH/LD=D0, CLK=D1, QH=D2), detects bit changes, emits `NoteOn` events
- `keymap.go` — `BitToNote[40]` mapping (C4–D#7), default channel/velocity
- `out.go` — sends NoteOn/Off/ProgramChange/Volume via UART (31250 baud) and BLE MIDI (Apple BLE MIDI service, 13-bit timestamp header)
- `server.go` — registers two BLE GATT services: standard MIDI service (UUID `03B8…`) and custom config service (UUID `12345678-…`); config characteristic write handler feeds `api.go`
- `api.go` — binary config protocol: `[cmd(1) | len_lo(1) | len_hi(1) | payload(N) | crc8(1)]`; commands `0x01` get_program, `0x02` set_program; CRC-8 poly `0x07`
- `config.go` — `ChannelConfig` stores instrument, volume, octave for each of 16 MIDI channels

**PWA (`pwa/`):**
- `ble.js` — Web Bluetooth wrapper (connect/disconnect/read/write)
- `api.js` — builds binary config messages, handles instrument list and UI events
- `index.html` — UI with instrument selector and volume/octave sliders for Melody/Chord/Bass channels

## Hardware Notes

- **Target:** XIAO BLE (Seeed Studio, nRF52840)
- **UART MIDI:** TX=D6, RX=D7 at 31250 baud
- **Shift registers:** 5 chained 74HC165N (40 bits); SH/LD=D0, CLK=D1, QH=D2; 1 µs inter-bit delay
- **BLE MIDI UUID:** `03B80E5A-EDE8-4B33-A751-6CE34EC4C700` (standard Apple MIDI over BLE)
- **Config service UUID:** `12345678-1234-5678-1234-567890abcdef` (custom)

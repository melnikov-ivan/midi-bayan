package main

import "embed"

//go:embed assets/test.txt assets/test.mid
var files embed.FS

func ReadFile() {
	data, err := files.ReadFile("assets/test.txt")
	if err != nil {
		println(err.Error())
		return
	}

	println(string(data))
}

func ReadTestMIDI() (*MIDIFile, error) {
	return ReadEmbeddedMIDI(files, "assets/test.mid")
}

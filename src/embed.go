package main

import (
	"embed"
)

//go:embed assets/test.txt
var files embed.FS

func ReadFile() {
	data, err := files.ReadFile("assets/test.txt")
	if err != nil {
		println(err.Error())
		return
	}

	println(string(data))

	for {
	}
}

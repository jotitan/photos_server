package main

import (
	"github.com/jotitan/photos_server/gui"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 7 {
		log.Fatal("Usage : <token> <server_url> <local_url> <port> <name> <height> <width>")
	}
	gui.Run(gui.CreateConfig(os.Args))
}

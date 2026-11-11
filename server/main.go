package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	static := flag.String("static", "../client/public", "directory of client files to serve")
	flag.Parse()

	world := NewWorld()
	go world.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", serveWS(world))
	mux.Handle("/", http.FileServer(http.Dir(*static)))

	log.Printf("listening on %s (static: %s)", *addr, *static)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	static := flag.String("static", "../client/public", "directory of client files to serve")
	mode := flag.String("mode", "ffa", "game mode: ffa (free-for-all) or teams")
	nTeams := flag.Int("teams", 2, "number of teams in teams mode (2-4)")
	flag.Parse()

	m := ModeFFA
	switch *mode {
	case "ffa":
	case "teams":
		m = ModeTeams
	default:
		log.Fatalf("unknown mode %q (want ffa or teams)", *mode)
	}
	world := NewWorld(m, *nTeams)
	if m == ModeTeams {
		log.Printf("mode: teams (%d)", world.nTeams)
	} else {
		log.Printf("mode: free-for-all")
	}
	go world.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", serveWS(world))
	mux.Handle("/", http.FileServer(http.Dir(*static)))

	log.Printf("listening on %s (static: %s)", *addr, *static)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

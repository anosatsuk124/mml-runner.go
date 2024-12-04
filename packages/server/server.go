package server

import (
	"github.com/anosatsuk124/mml-runner/packages/webui"
	"log"
	"net/http"
)

func Serve() {
	http.Handle("/", http.FileServer(http.FS(webui.DistDir)))

	for {
		log.Println("Listening server on :8080")
		err := http.ListenAndServe(":8080", nil)

		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}
}

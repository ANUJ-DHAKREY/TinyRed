package main

import (
	"log"
	"log/slog"
	"os"
	"tinyred/rdb"
	"tinyred/resp"
	"tinyred/server"
	"tinyred/store"
)

func main() {
	defaultConfig := server.GetDefaultConfig()
	config := server.GetConfig(defaultConfig)
	s, err := server.NewServer(
		config,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		&store.Store{Data: make(map[string]*resp.Entry)},
		rdb.Init(),
	)
	if err != nil {
		log.Fatal(err)
	}
	s.ListenAndServe()
}

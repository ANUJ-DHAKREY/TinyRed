package main

import (
	"log"
	"log/slog"
	"os"
	"tinyred/rdb"
	"tinyred/server"
	"tinyred/store"
)

func main() {
	defaultConfig, err := server.GetDefaultConfig()
	if err != nil {
		log.Fatal(err)
	}
	config := server.GetConfig(defaultConfig)
	s, err := server.NewServer(
		config,
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		store.New(),
		rdb.Init(),
	)
	if err != nil {
		log.Fatal(err)
	}
	//base, RDB, list, AOF, transaction, optimistic locking
	//pub-sub, sorted sets, geospatial commands,
	//replication, authentication, stream, bitmaps
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

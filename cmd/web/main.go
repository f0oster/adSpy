package main

import (
	"context"
	"flag"
	"log"

	"f0oster/adspy/config"
	"f0oster/adspy/database"
	"f0oster/adspy/web"
)

func main() {
	addr := flag.String("addr", ":8080", "Listen address for web server (e.g., :8080)")
	flag.Parse()

	adSpyConfig := config.LoadEnvConfig("settings.env")

	ctx := context.Background()
	db := database.NewDatabase(adSpyConfig.AdSpyDsn, adSpyConfig.ManagementDsn)
	if err := db.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	webServer := web.NewServer(db, *addr, adSpyConfig)
	log.Printf("Starting web interface at http://localhost%s", *addr)
	if err := webServer.Start(); err != nil {
		log.Fatalf("Web server error: %v", err)
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lusty/config"
	"lusty/internal/database"
	"lusty/internal/router"
	"lusty/pkg/cloudinary"
)

func main() {
	cfg := config.Load()
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	database.SeedAdmin(db)

	cloud, err := cloudinary.NewClientFromParams(cfg.Cloudinary.CloudName, cfg.Cloudinary.APIKey, cfg.Cloudinary.APISecret)
	if err != nil {
		log.Fatalf("cloudinary: %v", err)
	}

	engine := router.Setup(cfg, db, cloud)
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	go func() {
		log.Printf("server listening on :%s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server shutdown:", err)
	}
	fmt.Println("server stopped")
}

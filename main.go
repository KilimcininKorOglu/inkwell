package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/KilimcininKorOglu/inkwell/internal/config"
	"github.com/KilimcininKorOglu/inkwell/internal/crypto"
	"github.com/KilimcininKorOglu/inkwell/internal/dashboard"
	"github.com/KilimcininKorOglu/inkwell/internal/fetcher"
	"github.com/KilimcininKorOglu/inkwell/internal/models"
	"github.com/KilimcininKorOglu/inkwell/internal/parser"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Set via ldflags at build time.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("inkwell %s (commit: %s, built: %s)\n", version, commit, buildDate)
		return
	}

	log.Printf("=== Inkwell %s Started ===", version)

	cfg := config.Load()

	// Connect to database with retry loop
	log.Println("Waiting for database connection...")
	var db *gorm.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			sqlDB, _ := db.DB()
			if pingErr := sqlDB.Ping(); pingErr == nil {
				break
			}
		}
		log.Printf("Waiting for database... attempt %d/30", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to database after 30 attempts: %v", err)
	}
	log.Println("Database connected successfully.")

	// Initialize database tables
	models.InitDB(db)

	// Start background fetcher goroutine (iterates over all enabled domains)
	go func() {
		interval := time.Duration(cfg.FetchInterval) * time.Second
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Panic in fetch/parse loop: %v", r)
					}
				}()

				// Fetch all enabled domains from database
				var domains []models.Domain
				if err := db.Where("enabled = ?", true).Find(&domains).Error; err != nil {
					log.Printf("Error fetching domains: %v", err)
					return
				}

				if len(domains) == 0 {
					log.Println("No enabled domains configured. Add domains via the web UI.")
					return
				}

				for _, domain := range domains {
					log.Printf("Starting IMAP fetch cycle for domain: %s", domain.Name)

					// Decrypt IMAP password
					decrypted := domain
					if domain.IMAPPassword != "" && cfg.EncryptionKey != "" {
						plainPass, err := crypto.Decrypt(domain.IMAPPassword, cfg.EncryptionKey)
						if err != nil {
							log.Printf("Error decrypting password for domain %s: %v", domain.Name, err)
							continue
						}
						decrypted.IMAPPassword = plainPass
					}

					xmls, err := fetcher.FetchDMARCReports(&decrypted)
					if err != nil {
						log.Printf("Error fetching for domain %s: %v", domain.Name, err)
						continue
					}

					if len(xmls) > 0 {
						log.Printf("Fetched %d XML reports for %s. Parsing...", len(xmls), domain.Name)
						for _, x := range xmls {
							if parseErr := parser.ParseDMARCXML(db, x, domain.ID); parseErr != nil {
								log.Printf("Error parsing XML for domain %s: %v", domain.Name, parseErr)
							}
						}
					}
				}
				log.Println("Fetch cycle complete for all domains.")
			}()

			log.Printf("Sleeping for %d seconds until next check.", cfg.FetchInterval)
			time.Sleep(interval)
		}
	}()

	// Create dashboard router
	router, err := dashboard.NewRouter(db, "templates", "static", cfg.AdminUser, cfg.AdminPassword, cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	// Start HTTP server
	addr := ":" + cfg.Port
	log.Printf("Starting Inkwell Dashboard on http://0.0.0.0%s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

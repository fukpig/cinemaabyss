package main

import (
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// newReverseProxy creates a new reverse proxy for the given target URL.
func newReverseProxy(targetURL *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	// The default director in NewSingleHostReverseProxy is usually sufficient.
	// It sets req.URL.Scheme, req.URL.Host, and req.Host.
	// It also copies the Host header from the original request to X-Forwarded-Host
	// and sets req.Host to targetURL.Host.
	return proxy
}

func main() {
	// Get upstream URLs from environment variables
	monolithUpstream := os.Getenv("MONOLITH_URL")
	if monolithUpstream == "" {
		monolithUpstream = "http://localhost:8080" // Default for local dev if not set
		log.Printf("Warning: MONOLITH_URL not set, using default %s", monolithUpstream)
	}
	monolithTargetURL, err := url.Parse(monolithUpstream)
	if err != nil {
		log.Fatalf("Error parsing MONOLITH_URL (%s): %v", monolithUpstream, err)
	}
	monolithProxy := newReverseProxy(monolithTargetURL)

	moviesServiceUpstream := os.Getenv("MOVIES_SERVICE_URL")
	if moviesServiceUpstream == "" {
		moviesServiceUpstream = "http://localhost:8081" // Default for local dev if not set
		log.Printf("Warning: MOVIES_SERVICE_URL not set, using default %s", moviesServiceUpstream)
	}
	moviesTargetURL, err := url.Parse(moviesServiceUpstream)
	if err != nil {
		log.Fatalf("Error parsing MOVIES_SERVICE_URL (%s): %v", moviesServiceUpstream, err)
	}
	moviesProxy := newReverseProxy(moviesTargetURL)

	eventsServiceUpstream := os.Getenv("EVENTS_SERVICE_URL")
	if eventsServiceUpstream == "" {
		eventsServiceUpstream = "http://localhost:8082" // Default for local dev if not set
		log.Printf("Warning: EVENTS_SERVICE_URL not set, using default %s", eventsServiceUpstream)
	}
	eventsTargetURL, err := url.Parse(eventsServiceUpstream)
	if err != nil {
		log.Fatalf("Error parsing EVENTS_SERVICE_URL (%s): %v", eventsServiceUpstream, err)
	}
	eventsProxy := newReverseProxy(eventsTargetURL)

	// Seed the random number generator once at startup
	rand.Seed(time.Now().UnixNano())

	// Define the main request handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Proxy received request: %s %s %s", r.Method, r.Host, r.URL.Path)

		// Route based on path prefix
		if strings.HasPrefix(r.URL.Path, "/api/movies") {
			gradualMigrationStr := os.Getenv("GRADUAL_MIGRATION")
			moviesMigrationPercentStr := os.Getenv("MOVIES_MIGRATION_PERCENT")

			gradualMigrationEnabled := false
			if strings.ToLower(gradualMigrationStr) == "true" {
				gradualMigrationEnabled = true
			}

			if gradualMigrationEnabled {
				moviesMigrationPercent := 0 // Default to 0% to new service if parsing fails or out of range
				parsedPercent, err := strconv.Atoi(moviesMigrationPercentStr)
				if err == nil {
					if parsedPercent >= 0 && parsedPercent <= 100 {
						moviesMigrationPercent = parsedPercent
					} else {
						log.Printf("Warning: MOVIES_MIGRATION_PERCENT '%s' is out of range [0, 100]. Defaulting to 0%% for movies service.", moviesMigrationPercentStr)
						// moviesMigrationPercent remains 0
					}
				} else {
					log.Printf("Warning: Could not parse MOVIES_MIGRATION_PERCENT '%s'. Defaulting to 0%% for movies service. Error: %v", moviesMigrationPercentStr, err)
					// moviesMigrationPercent remains 0
				}

				randomNumber := rand.Intn(100) // Generates a random number between 0 and 99
				if randomNumber < moviesMigrationPercent {
					log.Printf("Routing to MOVIES_SERVICE_URL (gradual migration %d%%, roll: %d): %s", moviesMigrationPercent, randomNumber, moviesTargetURL)
					moviesProxy.ServeHTTP(w, r)
				} else {
					log.Printf("Routing to MONOLITH_URL (gradual migration %d%%, roll: %d, fallback): %s", moviesMigrationPercent, randomNumber, monolithTargetURL)
					monolithProxy.ServeHTTP(w, r)
				}
			} else {
				// Gradual migration is not enabled for /api/movies, route to monolith
				log.Printf("Routing to MONOLITH_URL (gradual migration disabled for movies): %s", monolithTargetURL)
				monolithProxy.ServeHTTP(w, r)
			}
		} else if strings.HasPrefix(r.URL.Path, "/api/events") {
			log.Printf("Routing to EVENTS_SERVICE_URL: %s", eventsTargetURL)
			eventsProxy.ServeHTTP(w, r)
		} else {
			log.Printf("Routing to MONOLITH_URL: %s", monolithTargetURL)
			monolithProxy.ServeHTTP(w, r)
		}
	})

	// Define the health check handler
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Strangler Fig Proxy is healthy"))
		log.Println("Health check request successful")
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000" // Default port
	}

	log.Printf("Proxy server starting on port %s", port)
	log.Printf("Proxying to Monolith: %s", monolithTargetURL)
	log.Printf("Proxying to Movies Service: %s", moviesTargetURL)
	log.Printf("Proxying to Events Service: %s", eventsTargetURL)
	log.Printf("Gradual migration for /api/movies enabled: %s", os.Getenv("GRADUAL_MIGRATION"))
	log.Printf("Movies migration percent for /api/movies: %s", os.Getenv("MOVIES_MIGRATION_PERCENT"))

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}

package web

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"f0oster/adspy/config"
	"f0oster/adspy/database"

	"github.com/f0oster/gontsd/resolve"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

// Server handles HTTP requests for the web interface.
type Server struct {
	db          *database.Database
	mux         *http.ServeMux
	addr        string
	sidResolver resolve.SIDResolver
}

// NewServer creates a new web server instance.
func NewServer(db *database.Database, addr string, cfg config.ADSpyConfiguration) *Server {
	// Create LDAP client for SID resolution
	// gontsd expects full URL with scheme
	ldapServer := cfg.DcFQDN
	if !strings.HasPrefix(ldapServer, "ldap://") && !strings.HasPrefix(ldapServer, "ldaps://") {
		ldapServer = "ldap://" + ldapServer + ":389"
	}
	log.Printf("Creating LDAP client: Server=%s, BaseDN=%s, BindDN=%s", ldapServer, cfg.BaseDN, cfg.Username)
	ldapClient, err := resolve.NewLDAPClient(resolve.LDAPConfig{
		Server:   ldapServer,
		BaseDN:   cfg.BaseDN,
		BindDN:   cfg.Username,
		Password: cfg.Password,
		UseTLS:   false,
	})

	var sidResolver resolve.SIDResolver
	if err != nil {
		log.Printf("Warning: Could not create LDAP client for SID resolution: %v", err)
		log.Printf("Falling back to well-known SID resolution only")
		sidResolver = resolve.WellKnownSIDResolver{}
	} else {
		sidResolver = resolve.ChainSIDResolver{
			Resolvers: []resolve.SIDResolver{
				resolve.WellKnownSIDResolver{},
				resolve.NewLDAPSIDResolver(ldapClient),
			},
		}
		log.Printf("LDAP SID resolver initialized successfully")
	}

	s := &Server{
		db:          db,
		mux:         http.NewServeMux(),
		addr:        addr,
		sidResolver: sidResolver,
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up all HTTP routes.
func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("GET /api/objects", s.handleListObjects)
	s.mux.HandleFunc("GET /api/objects/{id}", s.handleGetObject)
	s.mux.HandleFunc("GET /api/objects/{id}/timeline", s.handleGetObjectTimeline)
	s.mux.HandleFunc("GET /api/objects/{id}/versions/{usn}/changes", s.handleGetVersionChanges)
	s.mux.HandleFunc("POST /api/sddiff", s.handleSDDiff)
	s.mux.HandleFunc("GET /api/object-types", s.handleGetObjectTypes)

	// Static file serving for Svelte app
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Printf("Warning: Could not load embedded frontend: %v", err)
		return
	}

	fileServer := http.FileServer(http.FS(distFS))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// For SPA routing: serve index.html for non-file requests
		path := r.URL.Path

		// Try to open the file
		f, err := distFS.Open(path[1:]) // Remove leading slash
		if err != nil {
			// File doesn't exist, serve index.html for SPA routing
			r.URL.Path = "/"
		} else {
			f.Close()
		}

		fileServer.ServeHTTP(w, r)
	})
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	log.Printf("Starting web server on %s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

// Handler returns the HTTP handler for use with custom servers.
func (s *Server) Handler() http.Handler {
	return s.mux
}

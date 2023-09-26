package ramchi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	c "github.com/Etwodev/ramchi/config"
	"github.com/Etwodev/ramchi/middleware"
	"github.com/Etwodev/ramchi/router"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

var log zerolog.Logger

type Server struct {
	idle        chan struct{}
	middlewares []middleware.Middleware
	routers     []router.Router
	instance		*http.Server
}

func New() *Server {
	format := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006/01/02 15:04:05"}
	log = zerolog.New(format).With().Timestamp().Logger()

	err := c.New()
	if err != nil {
		log.Fatal().Str("Function", "New").Err(err).Msg("Unexpected error")
	}
	return &Server{}
}

func (s *Server) LoadRouter(routers []router.Router) {
	s.routers = append(s.routers, routers...)
}

func (s *Server) LoadMiddleware(middlewares []middleware.Middleware) {
	s.middlewares = append(s.middlewares, middlewares...)
}

func (s *Server) Start() {
	s.instance = &http.Server{Addr: fmt.Sprintf("%s:%s", c.Address(), c.Port()), Handler: s.handler()}
	log.Info().Str("Port", c.Port()).Str("Address", c.Address()).Bool("Experimental", c.Experimental()).Msg("Server started")

	s.idle = make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		if err := s.instance.Shutdown(context.Background()); err != nil {
			log.Warn().Str("Function", "Shutdown").Err(err).Msg("Server shutdown failed!")
		}
		close(s.idle)
	}()

	if err := s.instance.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal().Str("Function", "ListenAndServe").Err(err).Msg("Unexpected error")
	}

	<-s.idle

	log.Info().Str("Port", c.Port()).Str("Address", c.Address()).Bool("Experimental", c.Experimental()).Msg("Server stopped")
}

func Handle(err error, function string) {
	if err != nil {
		log.Error().Str("Function", function).Err(err).Msg("Unexpected error")
	}
}

func (s *Server) handler() *chi.Mux {
	m := chi.NewMux()
	for _, middleware := range s.middlewares {
		if middleware.Status() && (middleware.Experimental() == c.Experimental() || !middleware.Experimental()) {
			log.Info().Bool("Experimental", middleware.Experimental()).Bool("Status", middleware.Status()).Msg("Registering middleware")
			m.Use(middleware.Method())
		}
	}
	s.initMux(m)
	return m
}

func (s *Server) initMux(m *chi.Mux) {
	for _, router := range s.routers {
		if router.Status() {
			for _, r := range router.Routes() {
				if r.Status() && (r.Experimental() == c.Experimental() || !r.Experimental()) {
					log.Info().Bool("Experimental", r.Experimental()).Bool("Status", r.Status()).Str("Method", r.Method()).Str("Path", r.Path()).Msg("Registering route")
					m.Method(r.Method(), r.Path(), r.Handler())
				}
			}
		}
	}
}

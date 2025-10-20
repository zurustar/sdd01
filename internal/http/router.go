package http

import (
	"net/http"
	"strings"
)

type RouterConfig struct {
	Auth       *AuthHandler
	Users      *UserHandler
	Rooms      *RoomHandler
	Schedules  *ScheduleHandler
	Middleware []func(http.Handler) http.Handler
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	if cfg.Auth != nil {
		mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				methodNotAllowed(w, http.MethodPost)
				return
			}
			cfg.Auth.CreateSession(w, r)
		})
		mux.HandleFunc("/sessions/current", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				methodNotAllowed(w, http.MethodDelete)
				return
			}
			cfg.Auth.DeleteCurrentSession(w, r)
		})
		mux.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.URL.Path, "/sessions/")
			if token == "" {
				http.NotFound(w, r)
				return
			}
			if r.Method != http.MethodDelete {
				methodNotAllowed(w, http.MethodDelete)
				return
			}
			cfg.Auth.DeleteSession(w, r, token)
		})
	}

	if cfg.Schedules != nil {
		mux.HandleFunc("/schedules", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				cfg.Schedules.List(w, r)
			case http.MethodPost:
				cfg.Schedules.Create(w, r)
			default:
				methodNotAllowed(w, http.MethodGet, http.MethodPost)
			}
		})
		mux.HandleFunc("/schedules/", func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/schedules/")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			ctx := ContextWithScheduleID(r.Context(), id)
			r = r.WithContext(ctx)
			switch r.Method {
			case http.MethodPut:
				cfg.Schedules.Update(w, r)
			case http.MethodDelete:
				cfg.Schedules.Delete(w, r)
			default:
				methodNotAllowed(w, http.MethodPut, http.MethodDelete)
			}
		})
	}

	if cfg.Users != nil {
		mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				cfg.Users.List(w, r)
			case http.MethodPost:
				cfg.Users.Create(w, r)
			default:
				methodNotAllowed(w, http.MethodGet, http.MethodPost)
			}
		})
		mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/users/")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			ctx := ContextWithUserID(r.Context(), id)
			r = r.WithContext(ctx)
			switch r.Method {
			case http.MethodPut:
				cfg.Users.Update(w, r)
			case http.MethodDelete:
				cfg.Users.Delete(w, r)
			default:
				methodNotAllowed(w, http.MethodPut, http.MethodDelete)
			}
		})
	}

	if cfg.Rooms != nil {
		mux.HandleFunc("/rooms", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				cfg.Rooms.List(w, r)
			case http.MethodPost:
				cfg.Rooms.Create(w, r)
			default:
				methodNotAllowed(w, http.MethodGet, http.MethodPost)
			}
		})
		mux.HandleFunc("/rooms/", func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/rooms/")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			ctx := ContextWithRoomID(r.Context(), id)
			r = r.WithContext(ctx)
			switch r.Method {
			case http.MethodPut:
				cfg.Rooms.Update(w, r)
			case http.MethodDelete:
				cfg.Rooms.Delete(w, r)
			default:
				methodNotAllowed(w, http.MethodPut, http.MethodDelete)
			}
		})
	}

	var handler http.Handler = mux
	if len(cfg.Middleware) > 0 {
		for i := len(cfg.Middleware) - 1; i >= 0; i-- {
			if cfg.Middleware[i] != nil {
				handler = cfg.Middleware[i](handler)
			}
		}
	}

	return handler
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

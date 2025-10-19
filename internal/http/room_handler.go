package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/example/enterprise-scheduler/internal/application"
)

type roomService interface {
	CreateRoom(ctx context.Context, params application.CreateRoomParams) (application.Room, error)
	UpdateRoom(ctx context.Context, params application.UpdateRoomParams) (application.Room, error)
	DeleteRoom(ctx context.Context, principal application.Principal, roomID string) error
	ListRooms(ctx context.Context, principal application.Principal) ([]application.Room, error)
}

type RoomHandler struct {
	service   roomService
	responder responder
}

func NewRoomHandler(service roomService, logger *slog.Logger) *RoomHandler {
	return &RoomHandler{service: service, responder: newResponder(logger)}
}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	var req roomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	room, err := h.service.CreateRoom(r.Context(), application.CreateRoomParams{
		Principal: principal,
		Input:     req.toInput(),
	})
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusCreated, roomResponse{Room: toRoomDTO(room)})
}

func (h *RoomHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	roomID, ok := RoomIDFromContext(r.Context())
	if !ok || strings.TrimSpace(roomID) == "" {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidRoomID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())

	var req roomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errBadRequestBody)
		return
	}

	room, err := h.service.UpdateRoom(r.Context(), application.UpdateRoomParams{
		Principal: principal,
		RoomID:    roomID,
		Input:     req.toInput(),
	})
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusOK, roomResponse{Room: toRoomDTO(room)})
}

func (h *RoomHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	roomID, ok := RoomIDFromContext(r.Context())
	if !ok || strings.TrimSpace(roomID) == "" {
		h.responder.writeError(r.Context(), w, http.StatusBadRequest, errInvalidRoomID)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	if err := h.service.DeleteRoom(r.Context(), principal, roomID); err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusNoContent, nil)
}

func (h *RoomHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	principal, _ := PrincipalFromContext(r.Context())
	rooms, err := h.service.ListRooms(r.Context(), principal)
	if err != nil {
		h.responder.handleServiceError(r.Context(), w, err)
		return
	}

	h.responder.writeJSON(r.Context(), w, http.StatusOK, listRoomsResponse{Rooms: toRoomDTOs(rooms)})
}

type roomRequest struct {
	Name       string  `json:"name"`
	Location   string  `json:"location"`
	Capacity   int     `json:"capacity"`
	Facilities *string `json:"facilities"`
}

func (r roomRequest) toInput() application.RoomInput {
	var facilities *string
	if r.Facilities != nil {
		trimmed := strings.TrimSpace(*r.Facilities)
		facilities = &trimmed
	}
	return application.RoomInput{
		Name:       strings.TrimSpace(r.Name),
		Location:   strings.TrimSpace(r.Location),
		Capacity:   r.Capacity,
		Facilities: facilities,
	}
}

type roomResponse struct {
	Room roomDTO `json:"room"`
}

type listRoomsResponse struct {
	Rooms []roomDTO `json:"rooms"`
}

type roomDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Location   string  `json:"location"`
	Capacity   int     `json:"capacity"`
	Facilities *string `json:"facilities,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

func toRoomDTO(room application.Room) roomDTO {
	return roomDTO{
		ID:         room.ID,
		Name:       room.Name,
		Location:   room.Location,
		Capacity:   room.Capacity,
		Facilities: room.Facilities,
		CreatedAt:  room.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:  room.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toRoomDTOs(rooms []application.Room) []roomDTO {
	if len(rooms) == 0 {
		return nil
	}
	out := make([]roomDTO, 0, len(rooms))
	for _, room := range rooms {
		out = append(out, toRoomDTO(room))
	}
	return out
}

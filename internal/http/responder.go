package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/enterprise-scheduler/internal/application"
)

var (
	errBadRequestBody      = errors.New("無効なリクエスト形式です。")
	errInvalidScheduleID   = errors.New("無効なスケジュール ID です。")
	errInvalidUserID       = errors.New("無効なユーザー ID です。")
	errInvalidRoomID       = errors.New("無効な会議室 ID です。")
	errMissingSessionToken = errors.New("認証トークンを指定してください")
)

type responder struct {
	logger *slog.Logger
}

func newResponder(logger *slog.Logger) responder {
	if logger == nil {
		logger = slog.Default()
	}
	return responder{logger: logger}
}

func (r responder) writeJSON(ctx context.Context, w http.ResponseWriter, status int, payload any) {
	if w == nil {
		return
	}

	if status == http.StatusNoContent || payload == nil {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		r.loggerFor(ctx).ErrorContext(ctx, "failed to encode response", "error", err)
	}
}

func (r responder) writeError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	message := localizedStatusMessage(status)
	if err != nil {
		if msg := strings.TrimSpace(err.Error()); msg != "" {
			message = msg
		}
		r.loggerFor(ctx).ErrorContext(ctx, "request failed", "status", status, "error", err)
	}

	r.writeJSON(ctx, w, status, errorResponse{Message: message})
}

func (r responder) handleServiceError(ctx context.Context, w http.ResponseWriter, err error) {
	if err == nil {
		r.writeError(ctx, w, http.StatusInternalServerError, errors.New("unknown error"))
		return
	}

	switch {
	case errors.Is(err, application.ErrUnauthorized):
		r.writeJSON(ctx, w, http.StatusForbidden, errorResponse{
			ErrorCode: "AUTH_FORBIDDEN",
			Message:   "この操作を実行する権限がありません。",
		})
	case errors.Is(err, application.ErrNotFound):
		r.writeJSON(ctx, w, http.StatusNotFound, errorResponse{Message: "指定されたリソースが見つかりません。"})
	default:
		var vErr *application.ValidationError
		if errors.As(err, &vErr) {
			details := localizeValidationErrors(vErr)
			r.writeJSON(ctx, w, http.StatusUnprocessableEntity, errorResponse{
				Message: "入力内容に誤りがあります。",
				Errors:  details,
			})
			return
		}

		r.writeJSON(ctx, w, http.StatusInternalServerError, errorResponse{Message: "サーバー内部でエラーが発生しました。"})
	}
}

func (r responder) loggerFor(ctx context.Context) *slog.Logger {
	if logger := LoggerFromContext(ctx); logger != nil {
		return logger
	}
	return r.logger
}

func localizedStatusMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "リクエスト内容が正しくありません。"
	case http.StatusUnauthorized:
		return "認証が必要です。"
	case http.StatusForbidden:
		return "この操作を実行する権限がありません。"
	case http.StatusNotFound:
		return "指定されたリソースが見つかりません。"
	case http.StatusConflict:
		return "要求はリソースの現在の状態と競合しています。"
	case http.StatusUnprocessableEntity:
		return "入力内容に誤りがあります。"
	default:
		return "サーバー内部でエラーが発生しました。"
	}
}

func localizeValidationErrors(vErr *application.ValidationError) map[string]string {
	if vErr == nil || len(vErr.FieldErrors) == 0 {
		return nil
	}

	translated := make(map[string]string, len(vErr.FieldErrors))
	for field, msg := range vErr.FieldErrors {
		translated[field] = translateValidationMessage(msg)
	}
	return translated
}

func translateValidationMessage(message string) string {
	switch message {
	case "email is required":
		return "メールアドレスは必須です。"
	case "email is invalid":
		return "メールアドレスの形式が不正です。"
	case "display name is required":
		return "表示名は必須です。"
	case "name is required":
		return "会議室名は必須です。"
	case "location is required":
		return "所在地は必須です。"
	case "capacity must be positive":
		return "収容人数は正の整数で指定してください。"
	case "title is required":
		return "タイトルは必須です。"
	case "start is required":
		return "開始日時は必須です。"
	case "start must be in Asia/Tokyo (JST)":
		return "開始日時は日本標準時で指定してください。"
	case "end is required":
		return "終了日時は必須です。"
	case "end must be in Asia/Tokyo (JST)":
		return "終了日時は日本標準時で指定してください。"
	case "start must be before end":
		return "終了日時は開始日時より後である必要があります。"
	case "must be a valid URL":
		return "有効な URL を指定してください。"
	case "at least one participant is required":
		return "少なくとも 1 名の参加者を指定してください。"
	case "creator cannot be changed":
		return "作成者は変更できません。"
	case "room does not exist":
		return "指定された会議室は存在しません。"
	default:
		if strings.HasPrefix(message, "unknown user ids:") {
			return "存在しないユーザー ID が含まれています: " + strings.TrimSpace(strings.TrimPrefix(message, "unknown user ids:"))
		}
		return message
	}
}

type errorResponse struct {
	ErrorCode string            `json:"error_code,omitempty"`
	Message   string            `json:"message"`
	Errors    map[string]string `json:"errors,omitempty"`
}

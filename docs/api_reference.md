# API リファレンス（MVP）

Enterprise Scheduler の REST API 仕様をまとめる。すべてのエンドポイントは JSON を入出力し、
ベース URL は `https://{host}/api/v1` を想定する。レスポンス本文は UTF-8、時刻は JST (`YYYY-MM-DDTHH:MM:SS+09:00`).

## 共通事項

- 認証: `Authorization: Bearer {session_token}` ヘッダーを要求（`POST /sessions` で発行）。
- エラー形式:
  ```json
  {
    "error_code": "AUTH_INVALID_CREDENTIALS",
    "message": "メールアドレスまたはパスワードが正しくありません",
    "warnings": []
  }
  ```
- 日本語エラーコード一覧:
  | error_code | HTTP | 説明 |
  | --- | --- | --- |
  | `AUTH_INVALID_CREDENTIALS` | 401 | メールアドレスまたはパスワードが不正 |
  | `AUTH_SESSION_EXPIRED` | 401 | セッションの有効期限切れ |
  | `AUTH_FORBIDDEN` | 403 | 権限が不足 |
  | `SCHEDULE_NOT_FOUND` | 404 | スケジュールが存在しない |
  | `ROOM_NOT_FOUND` | 404 | 会議室が存在しない |
  | `VALIDATION_FAILED` | 422 | 入力検証エラー |
  | `CONFLICT_DETECTED` | 200 | 競合警告付き成功（レスポンス `warnings` に詳細） |
  | `INTERNAL_ERROR` | 500 | 予期せぬエラー |

- 競合警告: スケジュール関連レスポンスには `warnings` フィールド（配列）を含め、
  ```json
  {
    "type": "participant_overlap",
    "message": "参加者 Bob が 2024-05-10T10:00:00+09:00 に既存予定と重複しています",
    "participants": ["bob@example.com"],
    "room_id": null
  }
  ```
  の形式で返す。

## 認証

### `POST /sessions`
- 説明: メールアドレスとパスワードからセッショントークンを発行。
- リクエスト:
  ```json
  { "email": "alice@example.com", "password": "secret" }
  ```
- 成功レスポンス (201):
  ```json
  { "token": "sessiontoken", "expires_at": "2024-05-15T12:00:00+09:00" }
  ```
- 失敗レスポンス (401): `error_code=AUTH_INVALID_CREDENTIALS`。

### `DELETE /sessions/current`
- 説明: 現在のセッションを失効させる。
- レスポンス: 204 No Content。

## ユーザー情報

### `GET /me`
- 説明: ログイン中ユーザーのプロフィールと権限を取得。
- レスポンス (200):
  ```json
  {
    "id": "user-123",
    "email": "alice@example.com",
    "display_name": "Alice",
    "role": "employee",
    "timezone": "Asia/Tokyo"
  }
  ```

## スケジュール

### `GET /schedules`
- クエリ: `start`, `end`, `participants`, `rooms`。
- レスポンス (200): `items` 配列と `warnings`（フィルタに伴う警告）。

### `POST /schedules`
- リクエスト例:
  ```json
  {
    "title": "プロジェクトキックオフ",
    "start": "2024-05-10T10:00:00+09:00",
    "end": "2024-05-10T11:00:00+09:00",
    "participants": ["alice@example.com", "bob@example.com"],
    "room_id": "room-1",
    "online_url": "https://meet.example.com/xyz",
    "memo": "議題: MVP スコープ確認",
    "recurrence": {
      "type": "weekly",
      "weekdays": ["monday", "thursday"],
      "until": "2024-06-30"
    }
  }
  ```
- 成功レスポンス (201): `schedule` オブジェクトと `warnings`（競合がある場合）。
- バリデーション失敗 (422): `error_code=VALIDATION_FAILED`、`details` にフィールドごとのエラーメッセージ。

### `GET /schedules/{id}`
- 説明: 単一スケジュール取得。
- レスポンス (200): `schedule` オブジェクト、`warnings` は空配列。

### `PUT /schedules/{id}`
- 説明: 既存スケジュール更新（作成者または管理者のみ）。
- 成功 (200): 更新後の `schedule` と `warnings`。
- 権限不足 (403): `error_code=AUTH_FORBIDDEN`。

### `DELETE /schedules/{id}`
- 説明: スケジュール削除（作成者または管理者のみ）。
- 成功 (204)。

## 会議室

### `GET /rooms`
- 説明: 会議室一覧。
- レスポンス (200):
  ```json
  {
    "items": [
      {"id": "room-1", "name": "会議室A", "location": "5F", "capacity": 10, "facilities": ["プロジェクター"]}
    ]
  }
  ```

### `POST /rooms`
- 説明: 管理者が会議室を追加。
- リクエスト:
  ```json
  {"name": "会議室B", "location": "6F", "capacity": 8, "facilities": ["ホワイトボード"]}
  ```
- 成功 (201): 作成された部屋。
- 権限不足 (403)。

### `PUT /rooms/{id}` / `DELETE /rooms/{id}`
- 説明: 管理者のみ実行可能。
- `PUT` 成功 (200): 更新後オブジェクト。
- `DELETE` 成功 (204)。

## 競合検出

- `warnings` の `type` 値: `participant_overlap`, `room_overlap`。
- 競合検出 API は独立エンドポイントとして提供しない。`POST/PUT /schedules` のレスポンス内で返却。

## 管理用エンドポイント（MVP オプション）

### `GET /healthz`
- 説明: ランタイムヘルスチェック。認証不要。
- レスポンス (200): `{ "status": "ok" }`。

### `GET /metrics`
- 説明: Prometheus 形式のメトリクス。MVP 時点ではプレースホルダで 200 + 空ボディ。


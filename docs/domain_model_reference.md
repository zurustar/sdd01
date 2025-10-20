# ドメインモデルリファレンス

## ユーザー (User)
- **属性**
  - `id`: UUID 形式。
  - `email`: 会社ドメインメール。重複不可。
  - `display_name`: 50 文字以内。
  - `role`: `employee` または `administrator`。
  - `password_hash`: Argon2id 予定。
- **バリデーション**
  - メールは RFC 5322 のサブセット、会社ドメインのみ許容。
  - 表示名は全角半角混在可だが制御文字禁止。
- **センチネルエラー**
  - `ErrUserNotFound`
  - `ErrInvalidCredentials`

## 会議室 (Room)
- **属性**
  - `id`: `room-{n}`。
  - `name`: 64 文字以内、ユニーク。
  - `location`: 128 文字以内。
  - `capacity`: 正の整数。
  - `facilities`: 文字列配列、例: `"プロジェクター"`。
- **バリデーション**
  - 収容人数 1〜500。
  - 同名部屋は登録不可。
- **センチネルエラー**
  - `ErrRoomNotFound`
  - `ErrRoomDuplicateName`

## スケジュール (Schedule)
- **属性**
  - `id`: UUID。
  - `title`: 100 文字以内。
  - `start`, `end`: JST。`end > start`。
  - `participants`: ユーザー ID リスト。
  - `creator_id`: 作成者ユーザー ID。変更不可。
  - `room_id`: 任意。
  - `online_url`: 任意。`https://` or `http://`。
  - `memo`: Markdown テキスト（2,000 文字上限）。
  - `recurrence`: `RecurrenceRule` 参照。
- **バリデーション**
  - タイトル必須。
  - 参加者少なくとも 1 名。
  - 時刻は分解能 1 分。
- **センチネルエラー**
  - `ErrScheduleNotFound`
  - `ErrCreatorImmutable`
  - `ErrInvalidTimeRange`

## 繰り返し (RecurrenceRule)
- **属性**
  - `type`: `none` or `weekly`。
  - `weekdays`: `monday`〜`sunday` の配列（`weekly` の場合必須）。
  - `until`: 終了日 (YYYY-MM-DD)。
- **バリデーション**
  - `weekdays` は 1〜7 要素。
  - `until` は開始日より後。
- **センチネルエラー**
  - `ErrUnsupportedRecurrence`

## セッション (Session)
- **属性**
  - `token`: ランダム 32 byte。
  - `user_id`
  - `issued_at`, `expires_at`
  - `ip_address`, `user_agent`
- **バリデーション**
  - TTL デフォルト 12 時間。
  - `expires_at` までリフレッシュ不可（MVP）。
- **センチネルエラー**
  - `ErrSessionExpired`
  - `ErrSessionNotFound`

## 補助モデル
- **ConflictWarning**
  - `type`: `participant_overlap` or `room_overlap`
  - `message`: 日本語メッセージ
  - `participants`: 関連ユーザーID
  - `room_id`: 会議室 ID or `null`

- **AuditTrail (Backlog)**
  - MVP では未実装。将来の監査ログ用スキーマを想定。


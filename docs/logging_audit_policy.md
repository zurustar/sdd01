# ログ & 監査ポリシー

## `log/slog` フィールド規約
| フィールド | 説明 |
| --- | --- |
| `request_id` | HTTP ミドルウェアで生成。全ログに付与 |
| `user_id` | 認証済みの場合に付与 |
| `error_code` | センチネルエラーに対応 |
| `latency_ms` | ハンドラー処理時間 |
| `room_id` | 会議室関連操作時 |
| `schedule_id` | スケジュール関連操作時 |

## リクエスト ID の扱い
- `X-Request-ID` を優先採用。未指定なら UUID を生成。
- レスポンスヘッダーにもエコーバック。
- バックグラウンドジョブでは `job_id` を使用し、`request_id` は空文字でログ。

## センチネルエラー別ログレベル
| センチネル | レベル | 理由 |
| --- | --- | --- |
| `ErrInvalidCredentials` | `INFO` | 想定範囲内の失敗 |
| `ErrUnauthorized` | `WARN` | 不適切アクセスの可能性 |
| `ErrConflictDetected` | `INFO` | ビジネス警告 |
| `ErrScheduleNotFound` | `WARN` | 異常アクセスの兆候 |
| その他予期せぬエラー | `ERROR` | 即時調査 |

## 監査ログ拡張の留意点
- MVP ではアプリログのみ。将来的には `audit_events` テーブルを追加し、
  - `event_type`, `actor_id`, `target_type`, `target_id`, `metadata`, `occurred_at` を記録。
- 個人情報を含むフィールドは暗号化またはマスキング。
- ログ長期保存は 90 日、監査ログは 2 年を想定。


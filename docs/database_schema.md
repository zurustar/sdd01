# データベーススキーマとマイグレーション手順

SQLite (modernc.org/sqlite) を前提としたスキーマの初期ドラフト。将来的な RDBMS 移行を考慮して
外部キー制約を活用する。

## 主要テーブル

### `users`
| カラム | 型 | 制約 |
| --- | --- | --- |
| `id` | TEXT | PRIMARY KEY |
| `email` | TEXT | UNIQUE NOT NULL |
| `display_name` | TEXT | NOT NULL |
| `role` | TEXT | CHECK (role IN ('employee','administrator')) |
| `password_hash` | BLOB | NOT NULL |
| `created_at` | TEXT | DEFAULT CURRENT_TIMESTAMP |
| `updated_at` | TEXT | DEFAULT CURRENT_TIMESTAMP |

### `rooms`
| カラム | 型 | 制約 |
| --- | --- | --- |
| `id` | TEXT | PRIMARY KEY |
| `name` | TEXT | UNIQUE NOT NULL |
| `location` | TEXT | NOT NULL |
| `capacity` | INTEGER | CHECK (capacity > 0) |
| `facilities` | TEXT | JSON 文字列で保存 |
| `created_at` | TEXT | DEFAULT CURRENT_TIMESTAMP |
| `updated_at` | TEXT | DEFAULT CURRENT_TIMESTAMP |

### `schedules`
| カラム | 型 | 制約 |
| --- | --- | --- |
| `id` | TEXT | PRIMARY KEY |
| `title` | TEXT | NOT NULL |
| `start_time` | TEXT | NOT NULL |
| `end_time` | TEXT | CHECK (end_time > start_time) |
| `memo` | TEXT | NULL |
| `creator_id` | TEXT | NOT NULL REFERENCES users(id) |
| `room_id` | TEXT | NULL REFERENCES rooms(id) |
| `online_url` | TEXT | NULL |
| `created_at` | TEXT | DEFAULT CURRENT_TIMESTAMP |
| `updated_at` | TEXT | DEFAULT CURRENT_TIMESTAMP |

### `schedule_participants`
| カラム | 型 | 制約 |
| --- | --- | --- |
| `schedule_id` | TEXT | NOT NULL REFERENCES schedules(id) ON DELETE CASCADE |
| `user_id` | TEXT | NOT NULL REFERENCES users(id) |
| PRIMARY KEY (`schedule_id`, `user_id`) |

### `recurrences`
| カラム | 型 | 制約 |
| --- | --- | --- |
| `schedule_id` | TEXT | PRIMARY KEY REFERENCES schedules(id) ON DELETE CASCADE |
| `type` | TEXT | CHECK (type IN ('none','weekly')) |
| `weekdays` | TEXT | JSON 配列 |
| `until` | TEXT | NULL |

### `sessions`
| カラム | 型 | 制約 |
| --- | --- | --- |
| `token` | TEXT | PRIMARY KEY |
| `user_id` | TEXT | NOT NULL REFERENCES users(id) |
| `issued_at` | TEXT | NOT NULL |
| `expires_at` | TEXT | NOT NULL |
| `ip_address` | TEXT | NULL |
| `user_agent` | TEXT | NULL |

## インデックス
- `CREATE INDEX idx_schedules_start ON schedules(start_time);`
- `CREATE INDEX idx_schedules_room ON schedules(room_id, start_time);`
- `CREATE INDEX idx_participants_user ON schedule_participants(user_id);`
- `CREATE INDEX idx_sessions_user ON sessions(user_id);`

## CHECK 制約
- `rooms.capacity > 0`
- `schedules.end_time > schedules.start_time`
- `recurrences.type` が `weekly` の場合、`weekdays` JSON 配列長チェックはアプリ層で実施。

## マイグレーション手順
1. `internal/persistence/sqlite/migrations` ディレクトリに `<version>_<name>.up.sql/.down.sql` を配置。
2. Go 実装では `database/sql` + `modernc.org/sqlite` を使用し、`cmd/scheduler` 起動時に順次適用する。
3. 適用済みバージョンは `<dsn>.migrations.json` にタイムスタンプ付きで記録し、リトライ時は未適用分のみを実行する。
4. `scripts/backup.sh` 実行前にマイグレーション状態ファイルもバックアップ対象に含める。
5. ロールバックが必要な場合は対応する `down` ファイルを用意し、手動で適用する（MVP では up のみ運用）。

## バックアップ方針（MVP）
- `scripts/backup.sh` を cron に登録し、毎日 1 回バックアップ（保持 7 世代）を取得。
- Docker 運用時はボリュームをマウントしたホスト側でスクリプトを実行し、`.sqlite3.gz` を保管。
- 復旧手順: サービス停止 → `scripts/restore.sh` で指定のバックアップから復旧 → 再起動後に `GET /healthz` で確認。


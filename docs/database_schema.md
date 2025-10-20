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
1. `internal/storage/sqlite/migrations` ディレクトリにバージョン番号付き SQL ファイルを配置。
2. Go 実装では `database/sql` + `modernc.org/sqlite` を使用し、`db.Exec` でトランザクションを張って適用。
3. マイグレーション履歴テーブル `schema_migrations(version TEXT PRIMARY KEY, applied_at TEXT)` を作成。
4. 新しいマイグレーション適用時は:
   ```sql
   BEGIN;
   -- migration body
   INSERT INTO schema_migrations(version, applied_at) VALUES (?, CURRENT_TIMESTAMP);
   COMMIT;
   ```
5. ロールバックが必要な場合は対応する `down` ファイルを用意する（MVP では up のみ運用）。

## バックアップ方針（MVP）
- SQLite ファイルを 1 日 1 回停止時間中にコピー。
- Docker 運用時はボリュームをホストにマウントし、`sqlite3 .backup` コマンドで取得。
- 復旧手順: サービス停止 → バックアップファイルを置換 → サービス再起動 → `GET /healthz` で確認。


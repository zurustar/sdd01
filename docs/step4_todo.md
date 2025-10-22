# ステップ4 実装タスク計画

複数チームが並行して進められるよう、優先順位と依存関係の薄い作業単位で整理した。各グループは完了後にチェックを更新し、横断的な連携事項はメモ欄に追記していく。

## グループA: 認証・セッション基盤
- [v] Argon2id を用いたパスワード検証と資格情報取得フローを `internal/application.AuthService` とリポジトリに実装し、`POST /sessions` の成功・失敗レスポンス規約を満たす。【F:docs/authentication_authorization.md†L3-L36】【F:docs/enterprise_scheduler_spec.md†L47-L55】
  - [v] `POST /sessions` ハンドラーとルーターを仕様に合わせて再設計し、成功時 201 / 失敗時 401 を返すよう更新する。
  - [v] 資格情報取得アダプターと `AuthService` の Argon2id 検証を統合し、既存テストを補強する。
- [v] セッションリポジトリと `internal/http` のミドルウェアを接続し、期限切れ・無効トークン時に 401 を返す監査ログ付きの検証パスを整備する。【F:docs/authentication_authorization.md†L14-L41】【F:docs/step4_handoff.md†L11-L12】
  - [v] セッション検証ミドルウェアのエラーレスポンスを仕様の `error_code` / メッセージに合わせ、監査ログを調整する。
  - [v] ルーター初期化時に保護対象パスを整理し、ログイン API だけが匿名アクセス可能になるよう修正する。
- [v] ログアウト（トークン失効）と管理者向け失効 API を公開し、トークン漏洩リスクへの対策を強化する。【F:docs/authentication_authorization.md†L30-L36】【F:docs/step4_handoff.md†L11-L13】
  - [v] `DELETE /sessions/current` エンドポイントを追加し、クッキーおよびヘッダーからトークンを削除する。
  - [v] セッション失効を委譲するサービスメソッドとエラー時レスポンスのテストを追加する。

## グループB: 永続化・マイグレーション
- [v] (作業メモ) 既存の `internal/persistence` レイヤー構成と `database_schema.md` を精査し、リポジトリごとの永続化責務と期待されるクエリ境界を整理する。
  - [v] `internal/persistence` 配下の現状を調査し、主要コンポーネントと責務をメモ化する。
    - `errors.go` で永続化層共通のエラー表現を定義。`models.go` でドメインモデルを永続化向け構造体に正規化し、`repositories.go` がアプリケーション層と接続するインターフェース（ユーザー・会議室・スケジュール・繰り返し・セッション）を提供している。
    - `sqlite` 実装では `Storage` が各インターフェースを満たし、ユーザー／会議室／スケジュールなどの CRUD を一貫して提供。マップとミューテックスで整合性を確保し、`normalize*` ヘルパーやユニーク制約検証を内包している。
    - `repositories_test.go` でリポジトリ契約テストを整理済み。Step4 ではここを SQLite バックエンドと統合する方向性を維持する。
  - [v] `docs/database_schema.md` を読み、リポジトリごとのクエリ境界メモを追記する。
    - ユーザー／会議室は単純 CRUD + ユニーク制約が中心。`users` テーブルで `role` 列により権限を判定し、`rooms` は `name` ユニークと `capacity` チェックが要求される。
    - スケジュール関連は `schedules` 本体、`schedule_participants` 中間表、`recurrences` 付随表の結合が基本。`ScheduleFilter` は開始・終了範囲および参加者存在チェックを SQL 条件に落とし込む必要がある。
    - セッションは `sessions` テーブルで TTL と失効 (`revoked_at`) を管理。マイグレーションではインデックス整備と `schema_migrations` 管理が必須となる。
- [v] (作業メモ) SQLite の接続方法（`modernc.org/sqlite` ドライバ設定、PRAGMA）とトランザクションユーティリティ、`cmd/scheduler` 起動時のマイグレーション適用シーケンスを設計する。
  - [v] ドライバ設定と PRAGMA の整理案を `docs/step4_todo.md` のメモ欄に記述する。
    - `modernc.org/sqlite` ドライバを `sql.Open` で利用し、DSN は `file:...?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)` 形式に統一する。
    - 起動後に `PRAGMA cache_size=-2048`（2MB）と `PRAGMA synchronous=NORMAL` を発行し、耐障害性と性能のバランスを取る。テストではメモリ DB（`:memory:`）を利用して高速化する。
  - [v] マイグレーション適用シーケンス案を文書化する。
    - `internal/persistence/sqlite/migrations` に `<version>_<name>.up.sql/.down.sql` を配置し、`schema_migrations` をトランザクション内で管理。未適用バージョンのみ `BEGIN ... COMMIT` で順次適用する。
    - `cmd/scheduler` 起動時に 3 段階で実行: (1) DB 接続確立、(2) `Migrate` で up 適用、(3) 失敗時はリトライ（指数バックオフ最大 3 回）後にフェイル。ログは `logger.Info("sqlite migrate start", ...)` / `logger.Info("sqlite migrate done", ...)` で標準化する。
- [v] (作業メモ) バックアップ・復旧スクリプトの配置場所と CLI 仕様、運用ドキュメント更新内容をまとめる。
  - [v] バックアップ／復旧 CLI 方針をメモに書き出す。
    - `scripts/backup.sh` で `sqlite3 "$DB" ".backup $DEST"` を実行し、圧縮（`gzip`）と世代管理（保持 7 世代）を追加する。リストアは `scripts/restore.sh` で停止後に `.restore` を使う。
    - CLI では `cmd/scheduler-admin backup --output <path>` `restore --input <path>` を用意し、内部的に上記スクリプトを呼び出す構成を検討する。
  - [v] 運用ドキュメント更新案を整理する。
    - `docs/operations_runbook.md` にバックアップ取得／復旧手順、検証コマンド、失敗時のロールバック手順を追記。`docs/database_schema.md` にはスキーマ変更時にバックアップを取得するベストプラクティスを明記する。
- [v] `internal/persistence/sqlite` に接続プールと共通トランザクションヘルパーを実装し、`database/sql` エラーマッピングとログ出力方針を決める。【F:docs/database_schema.md†L3-L32】【F:docs/architecture_overview.md†L31-L52】
  - [v] SQLite コネクションマネージャとトランザクションヘルパーを実装する。
  - [v] エラーマッピングとログ方針を決定しコード化する。
- [v] ユーザー / 会議室リポジトリを SQLite 実装で提供し、ユニーク制約・外部キーを SQL レベルで検証する。【F:docs/database_schema.md†L33-L52】【F:docs/architecture_overview.md†L31-L52】
  - [v] ユーザーリポジトリの SQLite 実装とテストを追加する。
  - [v] 会議室リポジトリの SQLite 実装とテストを追加する。
- [v] スケジュール・参加者・会議室割当を管理するリポジトリを実装し、結合クエリと `ScheduleFilter` の範囲指定をカバーする。【F:docs/database_schema.md†L53-L78】【F:docs/scheduling_workflows.md†L3-L32】
  - [v] スケジュール／参加者／割当リポジトリを実装し、フィルタリングロジックを追加する。
  - [v] 結合クエリのテストを作成する。
- [v] 繰り返しルールとセッションリポジトリを実装し、TTL クリーンアップや作成・失効 API を支える SQL を整備する。【F:docs/database_schema.md†L33-L78】【F:docs/authentication_authorization.md†L14-L36】
  - [v] 繰り返しルールリポジトリとセッションリポジトリを実装する。
  - [v] TTL クリーンアップと失効 API 用 SQL を作成しテストする。
- [v] 初期スキーマを up/down マイグレーションスクリプトへ分解し、外部キー検証と索引付けを追加する。【F:docs/database_schema.md†L80-L95】
  - [v] 初期マイグレーションスクリプトを作成し、外部キーとインデックスを定義する。
- [v] マイグレーション runner を `cmd/scheduler` 起動シーケンスへ組み込み、起動ログとリトライポリシーを決める。【F:docs/database_schema.md†L80-L95】【F:docs/step4_handoff.md†L3-L7】
  - [v] マイグレーション runner を実装し、ログ／リトライ戦略を組み込む。
- [v] リポジトリとマイグレーションの結合テストを追加し、主要 CRUD / 参照整合性ケースをカバーする。【F:docs/test_strategy.md†L32-L54】
  - [v] 結合テストを実装して CRUD と参照整合性を検証する。
- [v] バックアップと復旧手順をスクリプト化し、運用ドキュメントへ反映する。【F:docs/database_schema.md†L93-L96】【F:docs/step4_handoff.md†L11-L13】
  - [v] バックアップ／復旧スクリプトを追加する。
  - [v] 運用ドキュメントを更新する。

## グループC: スケジュール・会議室ドメイン
- [v] (作業メモ) 繰り返しエンジンの性能検証手順と警告キャッシュの設計方針をまとめ、必要なコード／ドキュメントの変更点を洗い出す。
- [v] `internal/http/responder.go` にて日本語エラーレスポンスのマッピングを実装する。
- [v] `internal/application/schedule_service.go` の権限チェックを強化し、作成者・管理者以外の更新・削除を禁止する。
- [v] `internal/http/schedule_handler.go` で、作成者変更時に `403 Forbidden` を返すエラーハンドリングを追加する。
- [v] `internal/http/room_handler.go` および `internal/application/room_service.go` に管理者専用の会議室 CRUD API を実装する。
- [v] 繰り返し生成と競合検出ロジックに対するテストケースを追加し、仕様カバレッジを向上させる。
- [v] 繰り返しエンジンの性能検証と警告キャッシュ戦略を設計し、負荷時の応答遅延リスクを解消する。【F:docs/scheduling_workflows.md†L45-L54】【F:docs/step4_handoff.md†L11-L13】

## グループD: UI・ユーザードキュメント
- [v] (作業メモ) 週次プランナー UI モックの記述とスクリーンショット追記方法を整理する。
- [v] (作業メモ) マルチユーザー表示など UI 要件の補足説明方針を固める。
- [v] (作業メモ) UX ドキュメントとエラーメッセージのローカライズ項目の更新手順をまとめる。
- [v] 週次プランナー UI のモックと主要遷移を整備し、`docs/user_quickstart.md` にスクリーンショットを追加する。【F:docs/enterprise_scheduler_spec.md†L38-L75】【F:docs/step4_handoff.md†L3-L4】
- [v] マルチユーザー表示や会議室選択など UI 要件を満たすコンポーネントを実装し、仕様のビュー切り替え要件を検証する。【F:docs/enterprise_scheduler_spec.md†L38-L84】
- [v] UX ドキュメントとエラーメッセージのローカライズガイドラインを整備し、API との整合を図る。【F:docs/authentication_authorization.md†L30-L36】【F:docs/user_quickstart.md†L1-L80】

## グループE: DevOps・CI
- [v] GitHub Actions で lint / unit / race / coverage を実行するワークフローを追加し、Step4 開始時の CI 空白を解消する。【F:docs/documentation_plan.md†L40-L46】【F:docs/step4_handoff.md†L3-L6】
- [v] `golangci-lint` 設定ファイルとカバレッジ閾値チェックを導入し、テスト戦略のカバレッジ基準を自動化する。【F:docs/test_strategy.md†L15-L76】【F:docs/documentation_plan.md†L42-L44】
- [v] `CGO_ENABLED=0 go test ./...` を含むビルドパイプラインを構築し、SQLite ドライバ依存の回帰を防ぐ。【F:docs/documentation_plan.md†L40-L46】【F:docs/step4_handoff.md†L3-L13】

## グループF: クロスカッティング / オペレーション
- [ ] サービスロギングと監査出力ポリシーを `log/slog` へ実装し、エラー種別とリクエスト ID を標準化する。【F:docs/logging_audit_policy.md†L1-L88】
  - [v] `internal/http/middleware.go` の `RequestLogger` を修正し、`X-Request-ID` ヘッダーに対応し、なければUUIDを生成する。
  - [v] 認証済みリクエストのログに `user_id` を含める。
  - [v] `docs/logging_audit_policy.md` に従って、センチネルエラーに基づいたログレベルを設定する。
- [ ] 運用ランブックに沿ってヘルスチェック、アラート、バックアップ検証の自動化スクリプトを用意する。【F:docs/operations_runbook.md†L1-L120】【F:docs/step4_handoff.md†L11-L13】
- [ ] ドキュメント群（API リファレンス、テスト実行ガイド等）の更新を自動検証し、仕様とのトレーサビリティを維持する。【F:docs/traceability_matrix.md†L1-L120】【F:docs/documentation_plan.md†L6-L24】

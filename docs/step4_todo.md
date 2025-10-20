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
- [ ] (作業メモ) 既存の `internal/persistence` レイヤー構成と `database_schema.md` を精査し、リポジトリごとの永続化責務と期待されるクエリ境界を整理する。
- [ ] (作業メモ) SQLite の接続方法（`modernc.org/sqlite` ドライバ設定、PRAGMA）とトランザクションユーティリティ、`cmd/scheduler` 起動時のマイグレーション適用シーケンスを設計する。
- [ ] (作業メモ) バックアップ・復旧スクリプトの配置場所と CLI 仕様、運用ドキュメント更新内容をまとめる。
- [ ] `internal/persistence/sqlite` に接続プールと共通トランザクションヘルパーを実装し、`database/sql` エラーマッピングとログ出力方針を決める。【F:docs/database_schema.md†L3-L32】【F:docs/architecture_overview.md†L31-L52】
- [ ] ユーザー / 会議室リポジトリを SQLite 実装で提供し、ユニーク制約・外部キーを SQL レベルで検証する。【F:docs/database_schema.md†L33-L52】【F:docs/architecture_overview.md†L31-L52】
- [ ] スケジュール・参加者・会議室割当を管理するリポジトリを実装し、結合クエリと `ScheduleFilter` の範囲指定をカバーする。【F:docs/database_schema.md†L53-L78】【F:docs/scheduling_workflows.md†L3-L32】
- [ ] 繰り返しルールとセッションリポジトリを実装し、TTL クリーンアップや作成・失効 API を支える SQL を整備する。【F:docs/database_schema.md†L33-L78】【F:docs/authentication_authorization.md†L14-L36】
- [ ] 初期スキーマを up/down マイグレーションスクリプトへ分解し、外部キー検証と索引付けを追加する。【F:docs/database_schema.md†L80-L95】
- [ ] マイグレーション runner を `cmd/scheduler` 起動シーケンスへ組み込み、起動ログとリトライポリシーを決める。【F:docs/database_schema.md†L80-L95】【F:docs/step4_handoff.md†L3-L7】
- [ ] リポジトリとマイグレーションの結合テストを追加し、主要 CRUD / 参照整合性ケースをカバーする。【F:docs/test_strategy.md†L32-L54】
- [ ] バックアップと復旧手順をスクリプト化し、運用ドキュメントへ反映する。【F:docs/database_schema.md†L93-L96】【F:docs/step4_handoff.md†L11-L13】

## グループC: スケジュール・会議室ドメイン
- [ ] スケジュール CRUD、会議室管理、参加者選択、繰り返し生成、競合警告をアプリケーションサービスと HTTP ハンドラーに実装する。【F:docs/enterprise_scheduler_spec.md†L50-L113】【F:docs/scheduling_workflows.md†L3-L54】
- [ ] 仕様に沿った権限判定（作成者不変・管理者権限）とエラー変換を `internal/http` と `internal/application` に組み込み、日本語エラーコードを返す。【F:docs/enterprise_scheduler_spec.md†L14-L57】【F:docs/authentication_authorization.md†L22-L36】
- [ ] 繰り返しエンジンの性能検証と警告キャッシュ戦略を設計し、負荷時の応答遅延リスクを解消する。【F:docs/scheduling_workflows.md†L45-L54】【F:docs/step4_handoff.md†L11-L13】

## グループD: UI・ユーザードキュメント
- [v] (作業メモ) 週次プランナー UI モックの記述とスクリーンショット追記方法を整理する。
- [v] (作業メモ) マルチユーザー表示など UI 要件の補足説明方針を固める。
- [v] (作業メモ) UX ドキュメントとエラーメッセージのローカライズ項目の更新手順をまとめる。
- [v] 週次プランナー UI のモックと主要遷移を整備し、`docs/user_quickstart.md` にスクリーンショットを追加する。【F:docs/enterprise_scheduler_spec.md†L38-L75】【F:docs/step4_handoff.md†L3-L4】
- [v] マルチユーザー表示や会議室選択など UI 要件を満たすコンポーネントを実装し、仕様のビュー切り替え要件を検証する。【F:docs/enterprise_scheduler_spec.md†L38-L84】
- [v] UX ドキュメントとエラーメッセージのローカライズガイドラインを整備し、API との整合を図る。【F:docs/authentication_authorization.md†L30-L36】【F:docs/user_quickstart.md†L1-L80】

## グループE: DevOps・CI
- [ ] GitHub Actions で lint / unit / race / coverage を実行するワークフローを追加し、Step4 開始時の CI 空白を解消する。【F:docs/documentation_plan.md†L40-L46】【F:docs/step4_handoff.md†L3-L6】
- [ ] `golangci-lint` 設定ファイルとカバレッジ閾値チェックを導入し、テスト戦略のカバレッジ基準を自動化する。【F:docs/test_strategy.md†L15-L76】【F:docs/documentation_plan.md†L42-L44】
- [ ] `CGO_ENABLED=0 go test ./...` を含むビルドパイプラインを構築し、SQLite ドライバ依存の回帰を防ぐ。【F:docs/documentation_plan.md†L40-L46】【F:docs/step4_handoff.md†L3-L13】

## グループF: クロスカッティング / オペレーション
- [ ] サービスロギングと監査出力ポリシーを `log/slog` へ実装し、エラー種別とリクエスト ID を標準化する。【F:docs/logging_audit_policy.md†L1-L88】
- [ ] 運用ランブックに沿ってヘルスチェック、アラート、バックアップ検証の自動化スクリプトを用意する。【F:docs/operations_runbook.md†L1-L120】【F:docs/step4_handoff.md†L11-L13】
- [ ] ドキュメント群（API リファレンス、テスト実行ガイド等）の更新を自動検証し、仕様とのトレーサビリティを維持する。【F:docs/traceability_matrix.md†L1-L120】【F:docs/documentation_plan.md†L6-L24】

# Enterprise Scheduler Planning Repository

このリポジトリは Enterprise Scheduler MVP を仕様駆動で計画するためのドキュメントと補助ファイルを
管理する。ステップ3（ドキュメント作成とレビュー）の成果物として、主要ドキュメントとレビューノートを
整備した。

## 主要ドキュメント
- アーキテクチャ概要: [`docs/architecture_overview.md`](docs/architecture_overview.md)
- テスト戦略: [`docs/test_strategy.md`](docs/test_strategy.md)
- API リファレンス: [`docs/api_reference.md`](docs/api_reference.md)
- ドメインモデルリファレンス: [`docs/domain_model_reference.md`](docs/domain_model_reference.md)
- データベーススキーマ: [`docs/database_schema.md`](docs/database_schema.md)
- 設定・起動ガイド: [`docs/configuration_startup_guide.md`](docs/configuration_startup_guide.md)
- 認証・認可フロー: [`docs/authentication_authorization.md`](docs/authentication_authorization.md)
- スケジュールワークフロー: [`docs/scheduling_workflows.md`](docs/scheduling_workflows.md)
- テスト実行ガイド: [`docs/test_execution_guide.md`](docs/test_execution_guide.md)
- 運用ランブック: [`docs/operations_runbook.md`](docs/operations_runbook.md)
- ログ & 監査ポリシー: [`docs/logging_audit_policy.md`](docs/logging_audit_policy.md)
- 開発者オンボーディング: [`docs/developer_onboarding.md`](docs/developer_onboarding.md)
- 利用者クイックスタート: [`docs/user_quickstart.md`](docs/user_quickstart.md)
- トレーサビリティ表: [`docs/traceability_matrix.md`](docs/traceability_matrix.md)
- レビューノート: [`docs/documentation_review_notes.md`](docs/documentation_review_notes.md)

## ステップ3完了条件
- `docs/step3_todo.md` のチェックリストがすべて `[v]` で完了していること。
- トレーサビリティ表で仕様とテスト戦略のギャップが把握されていること。
- レビュー会で主要ステークホルダーから承認を得ていること（`docs/documentation_review_notes.md` を参照）。
- README が最新のドキュメント構成を案内していること。

## 次のステップ
- ステップ4では実装拡張と CI 構築を進め、レビューノートで挙げたフォローアップタスクを解決する。


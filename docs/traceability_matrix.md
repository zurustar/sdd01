# トレーサビリティ表（仕様・テスト・ドキュメント）

| 仕様セクション | 対応ドキュメント | テスト戦略 | 備考 |
| --- | --- | --- | --- |
| 3. Schedule Data Model | `docs/domain_model_reference.md`, `docs/database_schema.md` | `ScheduleService` ユニットテスト、リポジトリ統合テスト | 参加者・会議室属性を網羅 |
| 5. Core Functional Requirements - Authentication | `docs/authentication_authorization.md`, `docs/api_reference.md` | `AuthService` ユニットテスト | セッション TTL・401/403 の整理済み |
| 5. Core Functional Requirements - Schedule CRUD | `docs/scheduling_workflows.md`, `docs/api_reference.md` | `ScheduleService` / `ConflictDetector` テスト | 警告ハンドリングをテストケースに追加予定 |
| 5. Core Functional Requirements - Meeting Room Management | `docs/api_reference.md`, `docs/domain_model_reference.md` | `RoomService` ユニット + リポジトリ統合 | 管理者専用エンドポイントを明記 |
| 5. Core Functional Requirements - Recurrence | `docs/scheduling_workflows.md`, `docs/domain_model_reference.md` | `RecurrenceEngine` テスト | 週次のみ対応をテストで確認 |
| 6. Acceptance Criteria | `docs/user_quickstart.md`, `docs/scheduling_workflows.md` | API コンポーネントテスト計画 | UI 表現は将来の E2E で補完 |
| 7. UI/UX Considerations | `docs/user_quickstart.md` | Manual QA checklist | 日本語 UI での手順を記載 |
| テスト実行手順 | `docs/test_execution_guide.md` | - | CI で参照 |
| 運用要件 | `docs/operations_runbook.md`, `docs/logging_audit_policy.md` | - | 監視・ログ戦略を補完 |

ギャップ: 外部統合や通知はステップ4以降の Backlog に残置。パフォーマンス要件は未定義のため別途策定が必要。


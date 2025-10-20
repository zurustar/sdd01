# テスト実行ガイド

## コマンド一覧
| 目的 | コマンド | 備考 |
| --- | --- | --- |
| 単体テスト | `go test ./...` | ローカルで常用。`CGO_ENABLED=0` 可 |
| レース検知 | `go test -race ./...` | GitHub Actions の夜間ジョブで実行 |
| Lint | `golangci-lint run ./...` | `golangci.yml` をプロジェクト直下に配置予定 |
| カバレッジ | `go test -coverprofile=coverage.out ./...` | `go tool cover -html=coverage.out` で可視化 |

## フィクスチャ利用
- `internal/testfixtures` にユーザー、会議室、スケジュールのビルダーを配置予定。
- テストでは `fixtures.NewSchedule().WithParticipants(...)` のように構築。
- SQLite を用いた統合テストは一時ディレクトリに `scheduler_test.db` を作成し、テスト後に削除。

## 想定実行時間
- `go test ./...`: < 10 秒（コード規模が小さいうちは）。
- `go test -race ./...`: 30〜45 秒。
- `golangci-lint run`: 20 秒前後。

## トラブルシューティング
| 症状 | 対応 |
| --- | --- |
| SQLite ロックエラー | テスト並列実行を停止（`t.Parallel()` を外す）、一時 DB を個別に用意 |
| カバレッジファイル作成失敗 | `coverage.out` の書き込みパスが存在するか確認 |
| `golangci-lint` でのモジュール解決失敗 | `go env GOPRIVATE` 設定とプロキシを確認 |
| Race テストがタイムアウト | `-timeout=5m` を設定し、遅延テストをプロファイル |

## CI 手順
1. `go mod tidy` で依存関係を整理。
2. `go test ./...` を実行。
3. `go test -race ./...`（スモークでは省略可）。
4. `golangci-lint run ./...`。
5. カバレッジを収集してアーティファクト化。


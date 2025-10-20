# 開発者オンボーディングガイド

## 必要ツール
- Go 1.22+
- Docker / Docker Compose
- `golangci-lint`
- `direnv`（推奨）

## セットアップ手順
1. リポジトリをクローン: `git clone git@github.com:example/scheduler.git`。
2. `cp .env.example .env` を実行し、環境変数を設定。
3. `go mod download` で依存取得。
4. `make lint` / `make test`（Makefile はステップ4で整備予定）。

## サンプルデータ投入
- `scripts/seed.go`（予定）を `go run scripts/seed.go` で実行。
- SQLite ファイルにデモユーザー・会議室・予定を登録。

## デバッグ方法
- `go test ./... -run TestSpecific` で対象テストのみ実行。
- `LOG_LEVEL=debug` を設定して詳細ログを確認。
- VS Code Launch 設定（`.vscode/launch.json`）で API サーバにアタッチ。

## コーディング規約
- `gofmt` と `goimports` を保存時に実行。
- エラーは `fmt.Errorf("context: %w", err)` でラップ。
- センチネルエラーは `errors.Is` で比較し、HTTP レイヤーで適切にマッピング。
- パッケージ内部のみで利用する構造体は小文字で公開範囲を制限。

## Pull Request テンプレート
- 説明、テスト内容、スクリーンショット（該当時）を含める。
- 変更がドキュメントのみでもレビュワー 1 名の承認が必要。


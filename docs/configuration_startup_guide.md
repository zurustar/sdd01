# 設定・起動ガイド

## 環境変数
| 変数 | デフォルト | 説明 |
| --- | --- | --- |
| `HTTP_PORT` | `8080` | API サーバのポート |
| `SQLITE_DSN` | `file:data/scheduler.db?_pragma=foreign_keys(1)` | SQLite 接続文字列 |
| `SESSION_TTL_HOURS` | `12` | セッション有効期間 |
| `PASSWORD_HASH_MEMORY` | `64MB` | Argon2id メモリ設定 |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `REQUEST_TIMEOUT` | `15s` | HTTP タイムアウト |

## 実行コマンド
```bash
go run ./cmd/scheduler
```
- `internal/config` が環境変数を読み取り、`internal/http` が Echo/Gorilla 風のルーターを初期化。
- `internal/storage/sqlite` が DSN から接続し、マイグレーションを適用。

## ビルド（CGO 無効）
```bash
CGO_ENABLED=0 go build -o bin/scheduler ./cmd/scheduler
```
- `modernc.org/sqlite` は CGO 無効でもビルド可能。
- マルチステージ Docker ビルド時も同じ設定を利用。

## Docker イメージ
```dockerfile
FROM golang:1.22-bullseye AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o scheduler ./cmd/scheduler

FROM gcr.io/distroless/base-debian12
WORKDIR /srv
COPY --from=build /app/scheduler ./scheduler
CMD ["./scheduler"]
```

### Docker 起動手順
```bash
docker build -t enterprise-scheduler:dev .
docker run -p 8080:8080 -e SQLITE_DSN="file:/data/scheduler.db" \
  -v $(pwd)/data:/data enterprise-scheduler:dev
```

## ローカル開発フロー
1. `.env` ファイルに必要な値を設定し、`direnv` などで読み込む。
2. `go run ./cmd/scheduler --migrate`（仮オプション）でマイグレーション実行。
3. `GET http://localhost:8080/healthz` で稼働確認。


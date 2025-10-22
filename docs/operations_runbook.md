# 運用ランブック

## 監視項目
| カテゴリ | 指標 | 閾値 |
| --- | --- | --- |
| HTTP | 成功率 | 95% 未満で要調査 |
| HTTP | レイテンシ p95 | 800ms 超でアラート |
| ログ | `error` レベル件数 | 5 分間で 10 件以上 |
| SQLite | `PRAGMA integrity_check` | 異常検出時即時対応 |

## 障害初動
1. アラート受信後、直近デプロイ履歴を確認。
2. `kubectl logs` / `docker logs` でエラーを確認。
3. HTTP 5xx が継続する場合はローリングリスタート。
4. SQLite 整合性エラー時はリードオンリーに切り替え、バックアップから復旧準備。

## バックアップ / リストア
- バックアップ: 毎日 02:00 JST に `scripts/backup.sh -d /var/lib/scheduler/scheduler.db -o /var/backups/scheduler` を実行。`-k` で保持世代数（既定 7）を調整する。
- リストア: サービス停止 → `scripts/restore.sh -d /var/lib/scheduler/scheduler.db -i /var/backups/scheduler/scheduler.db.<timestamp>.sqlite3.gz` → `cmd/scheduler` を再起動し、起動時マイグレーションログを確認。
- 復旧後は `GET /healthz` と簡易スモークテストを実施し、最新バックアップの取得時刻を記録する。

## 連絡フロー
| 事象 | 連絡先 | SLA |
| --- | --- | --- |
| サービス停止 | on-call SRE → プロダクトオーナー | 15 分以内に初動報告 |
| データ消失懸念 | on-call SRE → セキュリティ担当 | 30 分以内に影響範囲共有 |
| 長時間性能劣化 | on-call SRE → テックリード | 1 時間以内に緩和策提示 |

## ポストモーテム
- 重大障害は 72 時間以内にポストモーテムを作成。
- 根本原因、再発防止、アクションアイテムを Backlog へ登録。


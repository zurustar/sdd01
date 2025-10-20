# 認証・認可フロー

## ログイン処理
1. ユーザーが `POST /sessions` にメール・パスワードを送信。
2. `AuthService` が `UserRepo` からユーザーを検索。
3. `bcrypt` ではなく Argon2id でパスワード検証。
4. 成功時に `SessionRepo` がトークンを生成し、`SESSION_TTL_HOURS` に基づき期限を設定。
5. クライアントへ `token` と `expires_at` を返す。

### エラー処理
- 認証失敗: `error_code=AUTH_INVALID_CREDENTIALS`, HTTP 401。
- ユーザーがロックされている場合（将来対応）: `AUTH_FORBIDDEN`。

## セッション検証ミドルウェア
- すべての認証必須エンドポイントで `Authorization` ヘッダーを検査。
- トークンが無効または期限切れの場合 `401` を返し、ボディは:
  ```json
  {"error_code": "AUTH_SESSION_EXPIRED", "message": "セッションの有効期限が切れています"}
  ```
- 成功時は `context` に `User` 情報を埋め込み後続ハンドラーへ渡す。

## 権限判定
- `role == administrator` のユーザーのみ:
  - 会議室 CRUD
  - 他者作成スケジュールの削除
- スケジュール更新・削除は `creator_id == current_user.id` または管理者の場合のみ許可。
- 参加者閲覧は全従業員が可能。
- 将来の細粒度権限は `PermissionService` で拡張予定。

## 失敗時レスポンス整形
| 状況 | HTTP | error_code | メッセージ例 |
| --- | --- | --- | --- |
| 未認証アクセス | 401 | `AUTH_SESSION_EXPIRED` | `セッションの有効期限が切れています` |
| 認証失敗 | 401 | `AUTH_INVALID_CREDENTIALS` | `メールアドレスまたはパスワードが正しくありません` |
| 権限不足 | 403 | `AUTH_FORBIDDEN` | `この操作を行う権限がありません` |
| セッション欠如 | 401 | `AUTH_SESSION_EXPIRED` | `認証トークンを指定してください` |

UI 上の文言は `docs/ux_localization_guidelines.md` のテーブルと同期し、トーン&マナーの変更は両方を同時に更新すること。

## セキュリティ備考
- セッションはデータベースに保存し、トークンはハッシュ化して保存予定（MVP ではプレーン GUID も可）。
- 失敗時レスポンスは常に同じ文言でタイミング攻撃を避ける。
- ログには `error_code` とリクエスト ID を出力。


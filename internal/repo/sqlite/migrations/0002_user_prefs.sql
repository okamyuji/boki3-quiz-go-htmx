-- user_prefs: ユーザーごとの学習画面設定 (セット/モード) を永続化する。
-- ログイン後の /quiz で復帰し、切り替え時に upsert される。
CREATE TABLE user_prefs (
  user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  quiz_set TEXT NOT NULL,
  quiz_mode TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

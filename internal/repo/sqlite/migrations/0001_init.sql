-- 0001_init.sql
-- 初回スキーマ。users / sessions / jwt_revocations / topics / questions / question_sets /
-- question_set_members / attempts / srs_states を作成する。
-- 時刻は unix epoch seconds (INTEGER) で保持する。

CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash BLOB NOT NULL,
  password_salt BLOB NOT NULL,
  password_params TEXT NOT NULL,
  password_updated_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_token TEXT NOT NULL,
  user_agent TEXT,
  ip TEXT,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE jwt_revocations (
  jti TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL,
  revoked_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX idx_jwt_rev_user ON jwt_revocations(user_id);
CREATE INDEX idx_jwt_rev_expires ON jwt_revocations(expires_at);

CREATE TABLE topics (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  ord INTEGER NOT NULL
);

CREATE TABLE questions (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  topic_id INTEGER NOT NULL REFERENCES topics(id),
  question_type TEXT NOT NULL,
  difficulty INTEGER NOT NULL,
  prompt TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  answer_json TEXT NOT NULL,
  explanation TEXT NOT NULL,
  references_json TEXT,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_questions_topic ON questions(topic_id);
CREATE INDEX idx_questions_type ON questions(question_type);

CREATE TABLE question_sets (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  target_size INTEGER NOT NULL
);

CREATE TABLE question_set_members (
  set_id INTEGER NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  ord INTEGER NOT NULL,
  PRIMARY KEY(set_id, question_id)
);
CREATE INDEX idx_qsm_question ON question_set_members(question_id);

CREATE TABLE attempts (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  set_id INTEGER REFERENCES question_sets(id) ON DELETE SET NULL,
  is_correct INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,
  submitted_answer_json TEXT NOT NULL,
  answered_at INTEGER NOT NULL
);
CREATE INDEX idx_attempts_user_time ON attempts(user_id, answered_at);
CREATE INDEX idx_attempts_user_q ON attempts(user_id, question_id);

CREATE TABLE srs_states (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  efactor REAL NOT NULL,
  interval_days INTEGER NOT NULL,
  repetitions INTEGER NOT NULL,
  due_at INTEGER NOT NULL,
  last_grade INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY(user_id, question_id)
);
CREATE INDEX idx_srs_due ON srs_states(user_id, due_at);

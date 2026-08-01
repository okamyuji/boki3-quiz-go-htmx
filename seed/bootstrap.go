package seed

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

//go:embed topics.json sets.json
var seedFS embed.FS

// TopicSeed は topics.json の 1 件。
type TopicSeed struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Ord  int    `json:"ord"`
}

// SetSeed は sets.json の 1 件。
type SetSeed struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TargetSize  int    `json:"target_size"`
}

// LoadTopics は embed.FS から topics を読み込む。
func LoadTopics() ([]TopicSeed, error) {
	b, err := seedFS.ReadFile("topics.json")
	if err != nil {
		return nil, fmt.Errorf("read topics: %w", err)
	}
	var out []TopicSeed
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse topics: %w", err)
	}
	return out, nil
}

// LoadSets は embed.FS から sets を読み込む。
func LoadSets() ([]SetSeed, error) {
	b, err := seedFS.ReadFile("sets.json")
	if err != nil {
		return nil, fmt.Errorf("read sets: %w", err)
	}
	var out []SetSeed
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse sets: %w", err)
	}
	return out, nil
}

// Sync は DB の問題バンクを現行の Generate() へ収束させる (idempotent)。
//
//   - topics / sets / questions を code キーで upsert する。
//   - 現行バンクに存在しない問題は、セット所属と SRS 状態を除去して
//     出題対象から外す。問題行と attempts は履歴表示のために保持する。
//
// アプリ起動時に毎回呼ばれる前提で、全体を 1 トランザクションで実行する。
func Sync(ctx context.Context, db *sql.DB) error {
	topics, err := LoadTopics()
	if err != nil {
		return err
	}
	sets, err := LoadSets()
	if err != nil {
		return err
	}
	questions := Generate()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := upsertTopicsTx(ctx, tx, topics); err != nil {
		return err
	}
	if err := upsertSetsTx(ctx, tx, sets); err != nil {
		return err
	}
	if err := upsertQuestionsTx(ctx, tx, questions); err != nil {
		return err
	}
	if err := retireStaleQuestionsTx(ctx, tx, questions); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sync commit: %w", err)
	}
	committed = true
	return nil
}

// retireStaleQuestionsTx は現行バンクに無い問題のセット所属と SRS 状態を削除する。
func retireStaleQuestionsTx(ctx context.Context, tx *sql.Tx, questions []Question) error {
	codes := make([]any, 0, len(questions))
	marks := make([]string, 0, len(questions))
	for i := range questions {
		codes = append(codes, questions[i].Code)
		marks = append(marks, "?")
	}
	in := strings.Join(marks, ",")
	staleFilter := `question_id IN (SELECT id FROM questions WHERE code NOT IN (` + in + `))`
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM question_set_members WHERE `+staleFilter, codes...); err != nil { //nolint:gosec // staleFilter は "?" プレースホルダのみで構成
		return fmt.Errorf("retire stale members: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM srs_states WHERE `+staleFilter, codes...); err != nil { //nolint:gosec // staleFilter は "?" プレースホルダのみで構成
		return fmt.Errorf("retire stale srs: %w", err)
	}
	return nil
}

func upsertTopicsTx(ctx context.Context, tx *sql.Tx, topics []TopicSeed) error {
	for _, t := range topics {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO topics(code, name, ord) VALUES (?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET name=excluded.name, ord=excluded.ord`,
			t.Code, t.Name, t.Ord); err != nil {
			return fmt.Errorf("upsert topic %s: %w", t.Code, err)
		}
	}
	return nil
}

func upsertSetsTx(ctx context.Context, tx *sql.Tx, sets []SetSeed) error {
	for _, s := range sets {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO question_sets(code, name, description, target_size) VALUES (?, ?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET name=excluded.name, description=excluded.description, target_size=excluded.target_size`,
			s.Code, s.Name, s.Description, s.TargetSize); err != nil {
			return fmt.Errorf("upsert set %s: %w", s.Code, err)
		}
	}
	return nil
}

func upsertQuestionsTx(ctx context.Context, tx *sql.Tx, questions []Question) error {
	now := time.Now().UTC().Unix()
	for i := range questions {
		q := &questions[i]
		topicID, err := lookupOne(ctx, tx, `SELECT id FROM topics WHERE code = ?`, q.TopicCode)
		if err != nil {
			return fmt.Errorf("topic %s missing: %w", q.TopicCode, err)
		}
		payload, err := json.Marshal(q.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload %s: %w", q.Code, err)
		}
		answer, err := json.Marshal(q.Answer)
		if err != nil {
			return fmt.Errorf("marshal answer %s: %w", q.Code, err)
		}
		refs, err := json.Marshal(q.References)
		if err != nil {
			return fmt.Errorf("marshal refs %s: %w", q.Code, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET
			   topic_id=excluded.topic_id,
			   question_type=excluded.question_type,
			   difficulty=excluded.difficulty,
			   prompt=excluded.prompt,
			   payload_json=excluded.payload_json,
			   answer_json=excluded.answer_json,
			   explanation=excluded.explanation,
			   references_json=excluded.references_json`,
			q.Code, topicID, q.QuestionType, q.Difficulty, q.Prompt,
			string(payload), string(answer), q.Explanation, string(refs), now); err != nil {
			return fmt.Errorf("upsert question %s: %w", q.Code, err)
		}
		qID, err := lookupOne(ctx, tx, `SELECT id FROM questions WHERE code = ?`, q.Code)
		if err != nil {
			return err
		}
		for _, setCode := range q.Sets {
			setID, err := lookupOne(ctx, tx, `SELECT id FROM question_sets WHERE code = ?`, setCode)
			if err != nil {
				return fmt.Errorf("set %s missing: %w", setCode, err)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO question_set_members(set_id, question_id, ord) VALUES (?, ?, 0)
				 ON CONFLICT(set_id, question_id) DO NOTHING`, setID, qID); err != nil {
				return fmt.Errorf("upsert member %s/%s: %w", setCode, q.Code, err)
			}
		}
	}
	return nil
}

func lookupOne(ctx context.Context, tx *sql.Tx, sqlText, code string) (int64, error) {
	var id int64
	if err := tx.QueryRowContext(ctx, sqlText, code).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

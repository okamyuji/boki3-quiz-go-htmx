// Package main は seed/* の JSON とジェネレータを使い、SQLite DB に topics / sets / questions を投入する。
//
// 使用例:
//
//	go run ./cmd/seed-loader -db ./boki3-quiz.db
//
// 既存の topics/sets/questions と code が一致するものは INSERT OR IGNORE で重複を避ける。
// question_set_members は (set_id, question_id) PRIMARY KEY で同様に重複を避ける。
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	reposqlite "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/seed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed loader failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "boki3-quiz.db", "SQLite DB path")
	seedDir := flag.String("seed", "seed", "Directory containing topics.json and sets.json")
	flag.Parse()

	db, err := sqlitex.Open("file:" + *dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := reposqlite.Migrate(ctx, db); err != nil {
		return err
	}

	topics, err := loadTopics(filepath.Join(*seedDir, "topics.json"))
	if err != nil {
		return err
	}
	if err := upsertTopics(ctx, db, topics); err != nil {
		return err
	}

	sets, err := loadSets(filepath.Join(*seedDir, "sets.json"))
	if err != nil {
		return err
	}
	if err := upsertSets(ctx, db, sets); err != nil {
		return err
	}

	questions := seed.Generate()
	if err := upsertQuestions(ctx, db, questions); err != nil {
		return err
	}
	fmt.Printf("seeded: topics=%d sets=%d questions=%d\n", len(topics), len(sets), len(questions))
	return nil
}

type topicSeed struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Ord  int    `json:"ord"`
}

type setSeed struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TargetSize  int    `json:"target_size"`
}

func loadTopics(path string) ([]topicSeed, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read topics: %w", err)
	}
	var out []topicSeed
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse topics: %w", err)
	}
	return out, nil
}

func loadSets(path string) ([]setSeed, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sets: %w", err)
	}
	var out []setSeed
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse sets: %w", err)
	}
	return out, nil
}

func upsertTopics(ctx context.Context, db *sql.DB, topics []topicSeed) error {
	for _, t := range topics {
		_, err := db.ExecContext(ctx,
			`INSERT INTO topics(code, name, ord) VALUES (?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET name=excluded.name, ord=excluded.ord`,
			t.Code, t.Name, t.Ord)
		if err != nil {
			return fmt.Errorf("upsert topic %s: %w", t.Code, err)
		}
	}
	return nil
}

func upsertSets(ctx context.Context, db *sql.DB, sets []setSeed) error {
	for _, s := range sets {
		_, err := db.ExecContext(ctx,
			`INSERT INTO question_sets(code, name, description, target_size) VALUES (?, ?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET name=excluded.name, description=excluded.description, target_size=excluded.target_size`,
			s.Code, s.Name, s.Description, s.TargetSize)
		if err != nil {
			return fmt.Errorf("upsert set %s: %w", s.Code, err)
		}
	}
	return nil
}

func upsertQuestions(ctx context.Context, db *sql.DB, questions []seed.Question) error {
	for _, q := range questions {
		topicID, err := lookupTopic(ctx, db, q.TopicCode)
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
		res, err := db.ExecContext(ctx,
			`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
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
			string(payload), string(answer), q.Explanation, string(refs))
		if err != nil {
			return fmt.Errorf("upsert question %s: %w", q.Code, err)
		}
		_ = res
		qID, err := lookupQuestion(ctx, db, q.Code)
		if err != nil {
			return err
		}
		for _, setCode := range q.Sets {
			setID, err := lookupSet(ctx, db, setCode)
			if err != nil {
				return fmt.Errorf("set %s missing: %w", setCode, err)
			}
			if _, err := db.ExecContext(ctx,
				`INSERT INTO question_set_members(set_id, question_id, ord) VALUES (?, ?, 0)
				 ON CONFLICT(set_id, question_id) DO NOTHING`, setID, qID); err != nil {
				return fmt.Errorf("upsert member %s/%s: %w", setCode, q.Code, err)
			}
		}
	}
	return nil
}

func lookupTopic(ctx context.Context, db *sql.DB, code string) (int64, error) {
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM topics WHERE code = ?`, code).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func lookupSet(ctx context.Context, db *sql.DB, code string) (int64, error) {
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM question_sets WHERE code = ?`, code).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func lookupQuestion(ctx context.Context, db *sql.DB, code string) (int64, error) {
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM questions WHERE code = ?`, code).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// ensure unused import not pruned
var _ domain.QuestionType

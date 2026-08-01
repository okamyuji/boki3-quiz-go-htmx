// Package main は seed パッケージの Sync を呼んで DB を現行の問題バンクへ収束させる CLI。
//
// 使用例:
//
//	go run ./cmd/seed-loader -db ./boki3-quiz.db
//
// Sync は idempotent (code キーで upsert し、バンク外の問題は出題対象から除外) なので、
// 開発中の再実行も安全。既存 DB を完全にリセットしたい場合は -force を指定する。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	reposqlite "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/seed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("seed loader failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("seed-loader", flag.ContinueOnError)
	dbPath := fs.String("db", "boki3-quiz.db", "SQLite DB path")
	force := fs.Bool("force", false, "Wipe topics/sets/questions/question_set_members before seeding")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := sqlitex.Open("file:" + *dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := reposqlite.Migrate(ctx, db); err != nil {
		return err
	}

	if *force {
		// 順序は FK 整合: question_set_members -> questions -> question_sets -> topics
		for _, q := range []string{
			`DELETE FROM question_set_members`,
			`DELETE FROM questions`,
			`DELETE FROM question_sets`,
			`DELETE FROM topics`,
		} {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("wipe: %w", err)
			}
		}
	}

	if err := seed.Sync(ctx, db); err != nil {
		return err
	}
	var nT, nS, nQ int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM topics`).Scan(&nT)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM question_sets`).Scan(&nS)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM questions`).Scan(&nQ)
	fmt.Printf("seeded: topics=%d sets=%d questions=%d\n", nT, nS, nQ)
	return nil
}

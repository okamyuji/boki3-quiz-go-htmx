// Package sqlite は domain port を SQLite database/sql で実装する。
package sqlite

import (
	"context"
	"database/sql"
	"embed"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
)

//go:embed all:migrations
var embeddedMigrations embed.FS

// Migrate は internal/repo/sqlite に embed されたマイグレーションを適用する。
//
// 実際の .sql ファイルは repo パッケージ内の migrations/ サブディレクトリへ複製してある。
// これは embed の制約 (相対パスのみ参照可) を満たすため。プロジェクトルートの
// migrations/ が単一ソースで、go generate もしくは make migrate-sync で同期される。
func Migrate(ctx context.Context, db *sql.DB) error {
	return sqlitex.Migrate(ctx, db, embeddedMigrations, "migrations")
}

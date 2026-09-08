package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MikhailMikryukov/NewsAggregator/internal/models"
)

type SourceRepository interface {
	SaveSource(ctx context.Context, rssURL string) error
	GetSources(ctx context.Context) ([]models.Source, error)
}

type ArticleRepository interface {
	SaveArticle(ctx context.Context, a models.Article, hash [16]byte) (int64, error)
	GetArticle(ctx context.Context, id int64) (*models.Article, error)
	UpdateArticle(ctx context.Context, a models.Article) error
	GetCountByTag(ctx context.Context, tag []string) (int, error)
	GetArticlesByTag(ctx context.Context, tag []string, offset int, limit int) ([]models.Article, error)
	GetAllTags(ctx context.Context) ([]string, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(ctx context.Context, connString string) (*PostgresRepository, error) {
	db, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresRepository{
		db: db,
	}, nil
}
func (r *PostgresRepository) SaveSource(ctx context.Context, rssURL string) error {
	query := "INSERT INTO sources (rss_url) VALUES ($1)"

	_, err := r.db.Exec(ctx, query, rssURL)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) GetSources(ctx context.Context) ([]models.Source, error) {
	query := "SELECT id, rss_url FROM sources"
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error getting sources %w", err)
	}
	defer rows.Close()

	sources := make([]models.Source, 0)
	for rows.Next() {
		var source models.Source

		err = rows.Scan(&source.ID, &source.RssURL)
		if err != nil {
			return nil, err
		}

		sources = append(sources, source)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return sources, nil
}

func (r *PostgresRepository) SaveArticle(ctx context.Context, a models.Article, hash [16]byte) (int64, error) {
	query := `
			INSERT INTO articles (source_id, original_url, title, content, tags, pub_date, status, hash) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) 
			ON CONFLICT (hash) DO NOTHING
			RETURNING id
			`

	var id int64
	err := r.db.QueryRow(ctx, query, a.SourceID, a.OriginalURL, a.Title, a.Content, a.Tags, a.PubDate, a.Status, hash[:]).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}

	return id, nil
}

func (r *PostgresRepository) GetArticle(ctx context.Context, id int64) (*models.Article, error) {
	query := "SELECT id, source_id, original_url, title, content, tags, pub_date, status FROM articles WHERE id = $1"

	row := r.db.QueryRow(ctx, query, id)
	var a models.Article

	err := row.Scan(&a.ID, &a.SourceID, &a.OriginalURL, &a.Title, &a.Content, &a.Tags, &a.PubDate, &a.Status)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

func (r *PostgresRepository) UpdateArticle(ctx context.Context, a models.Article) error {
	query := "UPDATE articles SET tags = $1, status = $2 WHERE id = $3"
	_, err := r.db.Exec(ctx, query, a.Tags, a.Status, a.ID)

	return err
}

func (r *PostgresRepository) GetCountByTag(ctx context.Context, tag []string) (int, error) {
	var query string
	var args []interface{}

	if len(tag) == 0 {
		query = "SELECT COUNT(*) FROM articles"
	} else {
		query = "SELECT COUNT(*) FROM articles WHERE tags && $1"
		args = append(args, tag)
	}

	var count int
	var err error

	if len(args) > 0 {
		err = r.db.QueryRow(ctx, query, args...).Scan(&count)
	} else {
		err = r.db.QueryRow(ctx, query).Scan(&count)
	}

	if err != nil {
		return -1, err
	}

	return count, nil
}

func (r *PostgresRepository) GetArticlesByTag(ctx context.Context, tag []string, offset int, limit int) ([]models.Article, error) {
	query := "SELECT id, source_id, original_url, title, content, tags, pub_date, status FROM articles"

	var args []interface{}

	if len(tag) != 0 {
		query += " WHERE tags in $1 OFFSET $2 LIMIT $3"
		args = []interface{}{tag, offset, limit}
	} else {
		query += " OFFSET $1 LIMIT $2"
		args = []interface{}{offset, limit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Article

	for rows.Next() {
		var a models.Article

		err = rows.Scan(&a.ID, &a.SourceID, &a.OriginalURL, &a.Title, &a.Content, &a.Tags, &a.PubDate, &a.Status)
		if err != nil {
			return nil, err
		}

		result = append(result, a)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return result, nil
}

func (r *PostgresRepository) GetAllTags(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT UNNEST(tags) FROM articles"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string

	for rows.Next() {
		var tag string

		err = rows.Scan(&tag)
		if err != nil {
			return nil, err
		}

		result = append(result, tag)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return result, nil
}

func (r *PostgresRepository) Close() {
	r.db.Close()
}

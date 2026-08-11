package repository

import (
	"NewsAggregator/internal/models"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ISourceArticleRepository interface {
	SaveSource(ctx context.Context, rssURL string) error
	GetSources(ctx context.Context) ([]models.Source, error)
	SaveArticle(ctx context.Context, a models.Article) (int64, error)
	GetArticle(ctx context.Context, id int64) (*models.Article, error)
	CheckArticleByHash(ctx context.Context, hash [16]byte) (bool, error)
	UpdateArticle(ctx context.Context, a models.Article) error
	GetCountByTag(ctx context.Context, tag []string) (int, error)
	GetArticlesByTag(ctx context.Context, tag []string, offset int) ([]models.Article, error)
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

func (r *PostgresRepository) SaveArticle(ctx context.Context, a models.Article) (int64, error) {
	query := "INSERT INTO articles (source_id, original_url, title, content, tags, status) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id"

	var id int64
	err := r.db.QueryRow(ctx, query, a.SourceID, a.OriginalURL, a.Title, a.Content, a.Tags, a.Status).Scan(&id)
	if err != nil {
		return -1, err
	}

	return id, nil
}

func (r *PostgresRepository) GetArticle(ctx context.Context, id int64) (*models.Article, error) {
	query := "SELECT id, source_id, original_url, title, content, tags, hash, status FROM articles WHERE id = $1"

	row := r.db.QueryRow(ctx, query, id)
	var a models.Article

	err := row.Scan(&a.ID, &a.SourceID, &a.OriginalURL, &a.Title, &a.Content, &a.Tags, &a.Hash, &a.Status)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

func (r *PostgresRepository) CheckArticleByHash(ctx context.Context, hash [16]byte) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM articles WHERE hash = $1)"
	row := r.db.QueryRow(ctx, query, hash)
	var exists bool

	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *PostgresRepository) UpdateArticle(ctx context.Context, a models.Article) error {
	query := "UPDATE articles SET tags, status VALUES ($1, $2) WHERE id = $3"
	_, err := r.db.Exec(ctx, query, a.Tags, a.Status, a.ID)

	return err
}

func (r *PostgresRepository) GetCountByTag(ctx context.Context, tag []string) (int, error) {
	query := "SELECT COUNT(*) FROM articles"

	if len(tag) != 0 {
		query += " WHERE tags in $1"
	}

	row := r.db.QueryRow(ctx, query)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return -1, err
	}

	return count, nil
}

func (r *PostgresRepository) GetArticlesByTag(ctx context.Context, tag []string, offset int) ([]models.Article, error) {
	query := "SELECT id, source_id, original_url, title, content, tags, hash, status FROM articles"

	var args []interface{}

	if len(tag) != 0 {
		query += " WHERE tags in $1 OFFSET $2"
		args = []interface{}{tag, offset}
	} else {
		query += " OFFSET $1"
		args = []interface{}{offset}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Article

	for rows.Next() {
		var a models.Article

		err = rows.Scan(&a.ID, &a.SourceID, &a.OriginalURL, &a.Title, &a.Content, &a.Tags, &a.Hash, &a.Status)
		if err != nil {
			return nil, err
		}

		result = append(result, a)
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

	return result, nil
}

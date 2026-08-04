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
	query := "INSERT INTO articles (source_id, original_url, content, tags, status) VALUES ($1, $2, $3, $4, $5) RETURNING id"

	var id int64
	err := r.db.QueryRow(ctx, query, a.SourceID, a.OriginalURL, a.Content, a.Tags, a.Status).Scan(&id)
	if err != nil {
		return -1, err
	}

	return id, nil
}

func (r *PostgresRepository) GetArticle(ctx context.Context, id int64) (*models.Article, error) {
	query := "SELECT id, source_id, original_url, content, tags, hash, status FROM articles WHERE id = $1"

	row := r.db.QueryRow(ctx, query, id)
	var a models.Article

	err := row.Scan(&a.ID, &a.SourceID, &a.OriginalURL, &a.Content, &a.Tags, &a.Hash, &a.Status)
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

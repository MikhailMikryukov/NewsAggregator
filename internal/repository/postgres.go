package repository

import (
	"NewsAggregator/internal/models"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type ISourceArticleRepository interface {
	SaveSource(ctx context.Context, rssURL string) error
	GetSources(ctx context.Context) ([]models.Source, error)
	SaveArticle(ctx context.Context, a models.Article) error
	GetArticle(ctx context.Context, id int) (*models.Article, error)
}

type PostgresRepository struct {
	db *pgx.Conn
}

func NewRepository(connString string) (*PostgresRepository, error) {
	db, err := pgx.Connect(context.Background(), connString)
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
	query := "SELECT * FROM sources"
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error getting sources %v", err)
	}

	sources := make([]models.Source, 0)
	for rows.Next() {
		var source models.Source

		err = rows.Scan(&source.ID, &source.RssURL)
		if err != nil {
			return nil, err
		}

		sources = append(sources, source)
	}

	return sources, nil
}

func (r *PostgresRepository) SaveArticle(ctx context.Context, a models.Article) error {
	query := "INSERT INTO articles (source_id, original_url, content, tags) VALUES ($1, $2, $3, $4)"

	_, err := r.db.Exec(ctx, query, a.SourceID, a.OriginalURL, a.Content, a.Tags)
	if err != nil {
		return err
	}

	return nil
}

func (r *PostgresRepository) GetArticle(ctx context.Context, id int) (*models.Article, error) {
	query := "SELECT * FROM articles WHERE id = $1"

	row := r.db.QueryRow(ctx, query, id)
	var a models.Article

	err := row.Scan(&a.ID, &a.SourceID, &a.OriginalURL, &a.Content, &a.Tags)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

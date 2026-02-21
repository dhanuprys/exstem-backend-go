package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

type SettingRepository struct {
	pool *pgxpool.Pool
}

func NewSettingRepository(pool *pgxpool.Pool) *SettingRepository {
	return &SettingRepository{pool: pool}
}

func (r *SettingRepository) GetAll(ctx context.Context) ([]model.AppSetting, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, value, updated_at FROM app_settings ORDER BY key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []model.AppSetting
	for rows.Next() {
		var s model.AppSetting
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (r *SettingRepository) Upsert(ctx context.Context, key, value string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		key, value)
	return err
}

func (r *SettingRepository) GetByKey(ctx context.Context, key string) (*model.AppSetting, error) {
	s := &model.AppSetting{}
	err := r.pool.QueryRow(ctx, `SELECT key, value, updated_at FROM app_settings WHERE key = $1`, key).
		Scan(&s.Key, &s.Value, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

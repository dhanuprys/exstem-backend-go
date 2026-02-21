package service

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/repository"
)

type SettingService struct {
	settingRepo *repository.SettingRepository
	log         zerolog.Logger
}

func NewSettingService(settingRepo *repository.SettingRepository, log zerolog.Logger) *SettingService {
	return &SettingService{
		settingRepo: settingRepo,
		log:         log.With().Str("component", "setting_service").Logger(),
	}
}

func (s *SettingService) GetAllSettings(ctx context.Context) (map[string]string, error) {
	settingsList, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to get all settings")
		return nil, err
	}

	settingsMap := make(map[string]string)
	for _, setting := range settingsList {
		settingsMap[setting.Key] = setting.Value
	}
	return settingsMap, nil
}

func (s *SettingService) UpdateSettings(ctx context.Context, settingsMap map[string]string) error {
	// Simple iterative upsert since settings are low volume. Can be optimized into a single tx if needed.
	for key, value := range settingsMap {
		if err := s.settingRepo.Upsert(ctx, key, value); err != nil {
			s.log.Error().Err(err).Str("key", key).Msg("failed to update setting")
			return err
		}
	}
	return nil
}

func (s *SettingService) GetSettingByKey(ctx context.Context, key string) (string, error) {
	setting, err := s.settingRepo.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

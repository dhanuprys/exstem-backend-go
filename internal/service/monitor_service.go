package service

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// MonitorService orchestrates live exam monitoring business logic.
type MonitorService struct {
	monitorRepo *repository.MonitorRepository
}

// NewMonitorService creates a new MonitorService.
func NewMonitorService(monitorRepo *repository.MonitorRepository) *MonitorService {
	return &MonitorService{monitorRepo: monitorRepo}
}

// StudentProgressSnapshot holds the answered count and cheat count for every in-progress student.
type StudentProgressSnapshot struct {
	AnsweredCounts map[int]int64 // student_id → answered_count
	CheatCounts    map[int]int64 // student_id → cheat_count
	TotalCheats    int64         // total cheats in the exam
}

// GetStudentProgress returns answered counts and cheat counts concurrently.
// It fires two independent data fetches in parallel to minimize latency.
func (s *MonitorService) GetStudentProgress(ctx context.Context, examID uuid.UUID) (*StudentProgressSnapshot, error) {
	snapshot := &StudentProgressSnapshot{
		AnsweredCounts: make(map[int]int64),
		CheatCounts:    make(map[int]int64),
	}

	var (
		answeredCounts map[int]int64
		cheatCounts    map[int]int64
		answeredErr    error
		cheatErr       error
		wg             sync.WaitGroup
	)

	// 1. Fetch answered counts directly from student_answers table
	wg.Add(1)
	go func() {
		defer wg.Done()
		answeredCounts, answeredErr = s.monitorRepo.GetAnsweredCounts(ctx, examID)
	}()

	// 2. Fetch cheat counts (single DB query, runs concurrently)
	wg.Add(1)
	go func() {
		defer wg.Done()
		cheatCounts, cheatErr = s.monitorRepo.GetCheatCounts(ctx, examID)
	}()

	wg.Wait()

	// Answered counts are critical; cheat counts are best-effort
	if answeredErr != nil {
		return nil, answeredErr
	}

	if answeredCounts != nil {
		snapshot.AnsweredCounts = answeredCounts
	}

	if cheatErr == nil && cheatCounts != nil {
		snapshot.CheatCounts = cheatCounts
		for _, count := range cheatCounts {
			snapshot.TotalCheats += count
		}
	}

	return snapshot, nil
}

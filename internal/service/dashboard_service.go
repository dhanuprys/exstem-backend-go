package service

import (
	"context"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// DashboardData consolidates all metrics for the admin dashboard.
type DashboardData struct {
	TotalStudents        int                                    `json:"total_students"`
	TotalExams           int                                    `json:"total_exams"`
	TotalQuestionBanks   int                                    `json:"total_question_banks"`
	TotalQuestions       int                                    `json:"total_questions"`
	ExamStatusCounts     map[model.ExamStatus]int               `json:"exam_status_counts"`
	UpcomingExams        []repository.DashboardUpcomingExam     `json:"upcoming_exams"`
	RecentCompletedExams []repository.DashboardRecentExamResult `json:"recent_completed_exams"`
}

// DashboardService handles admin dashboard business logic.
type DashboardService struct {
	repo *repository.DashboardRepository
}

// NewDashboardService creates a new DashboardService.
func NewDashboardService(repo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// GetDashboardData orchestrates fetching all dashboard metrics concurrently or sequentially.
func (s *DashboardService) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	students, exams, qbanks, questions, err := s.repo.GetSummaryCounts(ctx)
	if err != nil {
		return nil, err
	}

	statusCounts, err := s.repo.GetExamStatusCounts(ctx)
	if err != nil {
		return nil, err
	}

	upcoming, err := s.repo.GetUpcomingExams(ctx, 5)
	if err != nil {
		return nil, err
	}

	recent, err := s.repo.GetRecentExamResults(ctx, 5)
	if err != nil {
		return nil, err
	}

	data := &DashboardData{
		TotalStudents:        students,
		TotalExams:           exams,
		TotalQuestionBanks:   qbanks,
		TotalQuestions:       questions,
		ExamStatusCounts:     statusCounts,
		UpcomingExams:        upcoming,
		RecentCompletedExams: recent,
	}

	return data, nil
}

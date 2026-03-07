package service

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// ExamScheduleService handles auto-distribution mapping rules for rooms.
type ExamScheduleService struct {
	scheduleRepo *repository.ExamScheduleRepository
	roomRepo     *repository.RoomRepository
}

// NewExamScheduleService creates a new ExamScheduleService.
func NewExamScheduleService(scheduleRepo *repository.ExamScheduleRepository, roomRepo *repository.RoomRepository) *ExamScheduleService {
	return &ExamScheduleService{
		scheduleRepo: scheduleRepo,
		roomRepo:     roomRepo,
	}
}

// GetDistribution retrieves schedules and assignments for an exam.
func (s *ExamScheduleService) GetDistribution(ctx context.Context, examID uuid.UUID) (*model.DistributionResult, error) {
	schedules, err := s.scheduleRepo.ListByExam(ctx, examID)
	if err != nil {
		return nil, err
	}

	assignments, err := s.scheduleRepo.ListAssignments(ctx, examID)
	if err != nil {
		return nil, err
	}

	return &model.DistributionResult{
		Schedules:   schedules,
		Assignments: assignments,
	}, nil
}

// ClearDistribution removes all schedules for an exam.
func (s *ExamScheduleService) ClearDistribution(ctx context.Context, examID uuid.UUID) error {
	return s.scheduleRepo.DeleteByExam(ctx, examID)
}

// UpdateScheduleTime updates the start and end times for a specific schedule.
func (s *ExamScheduleService) UpdateScheduleTime(ctx context.Context, scheduleID uuid.UUID, req model.UpdateScheduleTimeRequest) error {
	return s.scheduleRepo.UpdateTime(ctx, scheduleID, req.StartTime, req.EndTime)
}

// AutoDistribute distributes students into exam rooms and sessions.
func (s *ExamScheduleService) AutoDistribute(ctx context.Context, examID uuid.UUID, req model.AutoDistributeRequest) error {
	// 1. Fetch eligible students based on source_mode
	var students []model.Student
	var err error

	switch req.SourceMode {
	case "target_rules":
		students, err = s.scheduleRepo.GetStudentsByTargetRules(ctx, examID)
	case "manual":
		students, err = s.scheduleRepo.GetStudentsByIDs(ctx, req.ClassIDs, req.StudentIDs)
	case "by_exam":
		if req.SourceExamID == nil {
			return errors.New("source_exam_id is required for by_exam mode")
		}
		students, err = s.scheduleRepo.GetStudentsByTargetRules(ctx, *req.SourceExamID)
	default:
		return errors.New("invalid source_mode")
	}

	if err != nil {
		return err
	}
	if len(students) == 0 {
		return errors.New("no eligible students found")
	}

	// 2. Fetch selected rooms to get their capacities
	var rooms []model.Room
	totalCapacity := 0
	for _, roomID := range req.RoomIDs {
		room, err := s.roomRepo.GetByID(ctx, roomID)
		if err != nil {
			return err
		}
		rooms = append(rooms, *room)
		totalCapacity += room.Capacity
	}

	if totalCapacity == 0 {
		return errors.New("selected rooms have zero total capacity")
	}

	// 3. Clear existing distribution
	if err := s.scheduleRepo.DeleteByExam(ctx, examID); err != nil {
		return err
	}

	// 4. Determine number of sessions needed
	requiredSessions := int(math.Ceil(float64(len(students)) / float64(totalCapacity)))

	// 5. Distribute students
	var schedulesToInsert []model.ExamSchedule
	var assignmentsToInsert []model.ExamRoomAssignment

	studentIndex := 0
	totalStudents := len(students)

	for sessionNumber := 1; sessionNumber <= requiredSessions; sessionNumber++ {
		// For each session, distribute students across rooms
		for _, room := range rooms {
			if studentIndex >= totalStudents {
				break
			}

			// Create the schedule for this session + room combination
			scheduleID := uuid.New()
			schedule := model.ExamSchedule{
				ID:            scheduleID,
				ExamID:        examID,
				SessionNumber: sessionNumber,
				RoomID:        room.ID,
			}
			schedulesToInsert = append(schedulesToInsert, schedule)

			// Assign students up to the room's capacity
			studentsAssignedToRoom := 0
			for studentsAssignedToRoom < room.Capacity && studentIndex < totalStudents {
				student := students[studentIndex]
				seatNumber := studentsAssignedToRoom + 1

				assignment := model.ExamRoomAssignment{
					ID:             uuid.New(),
					ExamScheduleID: scheduleID,
					StudentID:      student.ID,
					SeatNumber:     seatNumber,
				}
				assignmentsToInsert = append(assignmentsToInsert, assignment)

				studentsAssignedToRoom++
				studentIndex++
			}
		}
	}

	// 6. Bulk Insert
	return s.scheduleRepo.BulkCreate(ctx, schedulesToInsert, assignmentsToInsert)
}

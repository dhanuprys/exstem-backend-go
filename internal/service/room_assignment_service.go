package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/xuri/excelize/v2"
)

// RoomAssignmentService handles standalone student-to-room distribution.
type RoomAssignmentService struct {
	assignmentRepo *repository.RoomAssignmentRepository
	roomRepo       *repository.RoomRepository
}

// NewRoomAssignmentService creates a new RoomAssignmentService.
func NewRoomAssignmentService(assignmentRepo *repository.RoomAssignmentRepository, roomRepo *repository.RoomRepository) *RoomAssignmentService {
	return &RoomAssignmentService{
		assignmentRepo: assignmentRepo,
		roomRepo:       roomRepo,
	}
}

// GetDistribution retrieves all sessions and assignments.
func (s *RoomAssignmentService) GetDistribution(ctx context.Context) (*model.DistributionResult, error) {
	sessions, err := s.assignmentRepo.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	assignments, err := s.assignmentRepo.ListAssignments(ctx)
	if err != nil {
		return nil, err
	}

	return &model.DistributionResult{
		Sessions:    sessions,
		Assignments: assignments,
	}, nil
}

// ClearDistribution removes all sessions and assignments.
func (s *RoomAssignmentService) ClearDistribution(ctx context.Context) error {
	return s.assignmentRepo.ClearAll(ctx)
}

// UpdateSessionTimes updates start_time and end_time for sessions by session number.
func (s *RoomAssignmentService) UpdateSessionTimes(ctx context.Context, req model.UpdateSessionTimesRequest) error {
	return s.assignmentRepo.UpdateSessionTimes(ctx, req.Sessions)
}

// AutoDistribute distributes students into rooms and sessions.
// If ClassIDs/StudentIDs are empty, ALL students are distributed.
func (s *RoomAssignmentService) AutoDistribute(ctx context.Context, req model.AutoDistributeRequest) error {
	// 1. Fetch students.
	var students []model.Student
	var err error

	if len(req.ClassIDs) > 0 || len(req.StudentIDs) > 0 {
		students, err = s.assignmentRepo.GetStudentsByFilter(ctx, req.ClassIDs, req.StudentIDs)
	} else {
		students, err = s.assignmentRepo.GetAllStudents(ctx)
	}

	if err != nil {
		return err
	}
	if len(students) == 0 {
		return errors.New("no eligible students found")
	}

	// 2. Fetch selected rooms to get their capacities.
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

	// 3. Clear existing distribution.
	if err := s.assignmentRepo.ClearAll(ctx); err != nil {
		return err
	}

	// 4. Determine number of sessions needed.
	requiredSessions := int(math.Ceil(float64(len(students)) / float64(totalCapacity)))

	// 5. Distribute students across sessions × rooms.
	var sessionsToInsert []model.RoomSession
	var assignmentsToInsert []model.StudentRoomAssignment

	studentIndex := 0
	totalStudents := len(students)

	for sessionNumber := 1; sessionNumber <= requiredSessions; sessionNumber++ {
		for _, room := range rooms {
			if studentIndex >= totalStudents {
				break
			}

			sessionID := uuid.New()
			session := model.RoomSession{
				ID:            sessionID,
				SessionNumber: sessionNumber,
				RoomID:        room.ID,
			}
			sessionsToInsert = append(sessionsToInsert, session)

			studentsAssigned := 0
			for studentsAssigned < room.Capacity && studentIndex < totalStudents {
				student := students[studentIndex]
				seatNumber := studentsAssigned + 1

				assignment := model.StudentRoomAssignment{
					ID:            uuid.New(),
					RoomSessionID: sessionID,
					StudentID:     student.ID,
					SeatNumber:    seatNumber,
				}
				assignmentsToInsert = append(assignmentsToInsert, assignment)

				studentsAssigned++
				studentIndex++
			}
		}
	}

	// 6. Bulk insert.
	return s.assignmentRepo.BulkCreate(ctx, sessionsToInsert, assignmentsToInsert)
}

// ExportPresenceXLSX generates an excel sheet for the room presence.
func (s *RoomAssignmentService) ExportPresenceXLSX(ctx context.Context, sessionOk bool, sessionFilter int, roomOk bool, roomFilter int) ([]byte, error) {
	dist, err := s.GetDistribution(ctx)
	if err != nil {
		return nil, err
	}

	if sessionOk || roomOk {
		var filtered []model.RoomSession
		for _, sess := range dist.Sessions {
			keep := true
			if sessionOk && sess.SessionNumber != sessionFilter {
				keep = false
			}
			if roomOk && sess.RoomID != roomFilter {
				keep = false
			}
			if keep {
				filtered = append(filtered, sess)
			}
		}
		dist.Sessions = filtered
	}

	if len(dist.Sessions) == 0 {
		return nil, errors.New("no distribution data available to export")
	}

	f := excelize.NewFile()
	defer f.Close()

	firstSheetRenamed := false

	// Index sessions and group assignments.
	sessionMap := make(map[uuid.UUID]model.RoomSession)
	for _, sess := range dist.Sessions {
		sessionMap[sess.ID] = sess
	}
	assignmentsBySession := make(map[uuid.UUID][]model.StudentRoomAssignment)
	for _, a := range dist.Assignments {
		assignmentsBySession[a.RoomSessionID] = append(assignmentsBySession[a.RoomSessionID], a)
	}

	// Styles.
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	tableHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	cellStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	centerCellStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	signatureStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})

	for _, sess := range dist.Sessions {
		sheetName := fmt.Sprintf("Sesi %d - %s", sess.SessionNumber, sess.RoomName)
		if len(sheetName) > 31 {
			sheetName = sheetName[:31]
		}

		if !firstSheetRenamed {
			f.SetSheetName("Sheet1", sheetName)
			firstSheetRenamed = true
		} else {
			f.NewSheet(sheetName)
		}

		// Title block.
		f.SetCellValue(sheetName, "A1", "DAFTAR HADIR UJIAN")
		f.MergeCell(sheetName, "A1", "F1")
		f.SetCellStyle(sheetName, "A1", "F1", headerStyle)

		f.SetCellValue(sheetName, "A2", fmt.Sprintf("Sesi: %d", sess.SessionNumber))
		f.SetCellValue(sheetName, "A3", fmt.Sprintf("Ruangan: %s (Kapasitas: %d)", sess.RoomName, sess.RoomCapacity))

		// Table headers.
		f.SetCellValue(sheetName, "A5", "No")
		f.SetCellValue(sheetName, "B5", "NIS")
		f.SetCellValue(sheetName, "C5", "Nama Siswa")
		f.SetCellValue(sheetName, "D5", "Kelas")
		f.SetCellValue(sheetName, "E5", "Tanda Tangan")
		f.SetCellValue(sheetName, "F5", "")
		f.MergeCell(sheetName, "E5", "F5")
		f.SetCellStyle(sheetName, "A5", "F5", tableHeaderStyle)

		// Column widths.
		f.SetColWidth(sheetName, "A", "A", 4)
		f.SetColWidth(sheetName, "B", "B", 8)
		f.SetColWidth(sheetName, "C", "C", 39)
		f.SetColWidth(sheetName, "D", "D", 15)
		f.SetColWidth(sheetName, "E", "E", 12)
		f.SetColWidth(sheetName, "F", "F", 12)

		row := 6
		students := assignmentsBySession[sess.ID]
		for i, a := range students {
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), a.SeatNumber)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), a.StudentNIS)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), a.StudentName)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), a.ClassName)

			// Signature columns (odd/even staggered).
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), "")
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), "")

			// We stagger the signature based on the row index (i) for aesthetic
			// or we can stagger based on SeatNumber. Using i makes it reliable visually descending.
			if (i+1)%2 == 1 {
				f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("%d.", a.SeatNumber))
			} else {
				f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), fmt.Sprintf("%d.", a.SeatNumber))
			}

			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), centerCellStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), centerCellStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("C%d", row), fmt.Sprintf("D%d", row), cellStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row), signatureStyle)

			f.SetRowHeight(sheetName, row, 30)
			row++
		}

		// Provide a bit of spacing before the signature
		row += 2

		// Add "Pengawas Ruangan TTD" string mapping
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), "Pengawas Ruangan,")

		f.SetRowHeight(sheetName, row+1, 30)
		f.SetRowHeight(sheetName, row+2, 30)

		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row+3), "(.........................................)")
	}

	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

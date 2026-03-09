package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/xuri/excelize/v2"
)

// RoomAssignmentService handles standalone student-to-room distribution.
type RoomAssignmentService struct {
	assignmentRepo *repository.RoomAssignmentRepository
	roomRepo       *repository.RoomRepository
	settingService *SettingService
}

// NewRoomAssignmentService creates a new RoomAssignmentService.
func NewRoomAssignmentService(assignmentRepo *repository.RoomAssignmentRepository, roomRepo *repository.RoomRepository, settingService *SettingService) *RoomAssignmentService {
	return &RoomAssignmentService{
		assignmentRepo: assignmentRepo,
		roomRepo:       roomRepo,
		settingService: settingService,
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

	// Fetch letterhead setting.
	letterheadURL, err := s.settingService.GetSettingByKey(ctx, "letterhead_url")
	var letterheadBytes []byte
	var letterheadExt string
	var letterheadWidth, letterheadHeight float64
	hasLetterhead := false

	if err == nil && letterheadURL != "" {
		// e.g., letterheadURL = "/uploads/media/kop.png"
		basePath := strings.TrimPrefix(letterheadURL, "/uploads")
		localPath := filepath.Join(".", "uploads", basePath)

		imgData, err := os.ReadFile(localPath)
		if err == nil {
			letterheadBytes = imgData
			letterheadExt = filepath.Ext(localPath)
			if letterheadExt == "" {
				letterheadExt = ".png"
			}

			imgCfg, _, err := image.DecodeConfig(bytes.NewReader(imgData))
			if err == nil && imgCfg.Width > 0 && imgCfg.Height > 0 {
				letterheadWidth = float64(imgCfg.Width)
				letterheadHeight = float64(imgCfg.Height)
			} else {
				// Default fallback width if decode fails
				letterheadWidth = 660.0
				letterheadHeight = 100.0
			}

			hasLetterhead = true
		} else {
			// Log silently, proceed without letterhead
			fmt.Println("Warning: Could not read letterhead file:", err)
		}
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

		baseRow := 1
		if hasLetterhead {
			// In Excelize, merged columns A-F with sizes (4,8,39,15,12,12) evaluate to exactly 583 pixels wide internally based on EMU calculations.
			targetWidth := 583.0

			// Compute the exact proportional height in pixels for a 583px wide image
			scaledHeightPixels := targetWidth * (letterheadHeight / letterheadWidth)

			// Excel row heights are specified in "points", where 1 pixel ≈ 0.75 points
			rowHeightPoints := scaledHeightPixels * 0.75

			// Merge cells A1 to F1 FIRST so AutoFit maps to the entire table width precisely
			f.MergeCell(sheetName, "A1", "F1")

			// Dynamically stretch Row 1 down to perfectly fit the scaled image height natively, preventing underlaps.
			f.SetRowHeight(sheetName, 1, rowHeightPoints)

			// Relinquish absolute dimension control back to AutoFit which scales width flawlessly avoiding resolution overlaps.
			imgFmt := &excelize.GraphicOptions{
				AutoFit:         true,
				LockAspectRatio: true,
				ScaleY:          1.25,
			}
			f.AddPictureFromBytes(sheetName, "A1", &excelize.Picture{
				Extension: letterheadExt,
				File:      letterheadBytes,
				Format:    imgFmt,
			})

			// Leave exactly two blank rows after the letterhead (Row 2, Row 3)
			baseRow = 2 // no extra blank space
		}

		// Title block.
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", baseRow), "DAFTAR HADIR UJIAN")
		f.MergeCell(sheetName, fmt.Sprintf("A%d", baseRow), fmt.Sprintf("F%d", baseRow))
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", baseRow), fmt.Sprintf("F%d", baseRow), headerStyle)

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", baseRow+1), fmt.Sprintf("Sesi: %d", sess.SessionNumber))
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", baseRow+2), fmt.Sprintf("Ruangan: %s (Kapasitas: %d)", sess.RoomName, sess.RoomCapacity))

		// Table headers.
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", baseRow+4), "No")
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", baseRow+4), "NIS")
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", baseRow+4), "Nama Siswa")
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", baseRow+4), "Kelas")
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", baseRow+4), "Tanda Tangan")
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", baseRow+4), "")
		f.MergeCell(sheetName, fmt.Sprintf("E%d", baseRow+4), fmt.Sprintf("F%d", baseRow+4))
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", baseRow+4), fmt.Sprintf("F%d", baseRow+4), tableHeaderStyle)

		// Column widths.
		f.SetColWidth(sheetName, "A", "A", 4)
		f.SetColWidth(sheetName, "B", "B", 8)
		f.SetColWidth(sheetName, "C", "C", 39)
		f.SetColWidth(sheetName, "D", "D", 15)
		f.SetColWidth(sheetName, "E", "E", 12)
		f.SetColWidth(sheetName, "F", "F", 12)

		row := baseRow + 5
		students := assignmentsBySession[sess.ID]
		for i, a := range students {
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), a.SeatNumber)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), a.StudentNIS)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), a.StudentName)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), fmt.Sprintf("  %s", a.ClassName))

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

			f.SetRowHeight(sheetName, row, 18)
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

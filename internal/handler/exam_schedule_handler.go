package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
	"github.com/xuri/excelize/v2"
)

type ExamScheduleHandler struct {
	scheduleService *service.ExamScheduleService
	examService     *service.ExamService
}

func NewExamScheduleHandler(scheduleService *service.ExamScheduleService, examService *service.ExamService) *ExamScheduleHandler {
	return &ExamScheduleHandler{
		scheduleService: scheduleService,
		examService:     examService,
	}
}

// AutoDistribute godoc
// POST /api/v1/admin/exams/:id/distribute
func (h *ExamScheduleHandler) AutoDistribute(c *gin.Context) {
	examIDStr := c.Param("id")
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.AutoDistributeRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	if err := h.scheduleService.AutoDistribute(c.Request.Context(), examID, req); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Error: &response.ErrorBody{
				Code:    response.ErrInternal,
				Message: err.Error(),
			},
		})
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "Students distributed successfully"})
}

// GetDistribution godoc
// GET /api/v1/admin/exams/:id/distribution
func (h *ExamScheduleHandler) GetDistribution(c *gin.Context) {
	examIDStr := c.Param("id")
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	res, err := h.scheduleService.GetDistribution(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, res)
}

// ClearDistribution godoc
// DELETE /api/v1/admin/exams/:id/distribution
func (h *ExamScheduleHandler) ClearDistribution(c *gin.Context) {
	examIDStr := c.Param("id")
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.scheduleService.ClearDistribution(c.Request.Context(), examID); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "Distribution cleared successfully"})
}

// UpdateScheduleTime godoc
// PUT /api/v1/admin/exam-schedules/:scheduleId/time
func (h *ExamScheduleHandler) UpdateScheduleTime(c *gin.Context) {
	scheduleIDStr := c.Param("scheduleId")
	scheduleID, err := uuid.Parse(scheduleIDStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.UpdateScheduleTimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrValidation)
		return
	}

	// Update times via repository method (already created in repo/service)
	if err := h.scheduleService.UpdateScheduleTime(c.Request.Context(), scheduleID, req); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "Schedule time updated successfully"})
}

// ExportPresenceXLSX godoc
// GET /api/v1/admin/exams/:id/distribution/export
func (h *ExamScheduleHandler) ExportPresenceXLSX(c *gin.Context) {
	examIDStr := c.Param("id")
	examID, err := uuid.Parse(examIDStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	exam, err := h.examService.GetByID(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrNotFound)
		return
	}

	dist, err := h.scheduleService.GetDistribution(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	// Optional filtering by session and/or room
	sessionFilterStr := c.Query("session")
	roomFilterStr := c.Query("room")

	if sessionFilterStr != "" || roomFilterStr != "" {
		var sessionFilter int
		var roomFilter int

		sessionOk := false
		if sessionFilterStr != "" {
			_, err := fmt.Sscanf(sessionFilterStr, "%d", &sessionFilter)
			sessionOk = err == nil
		}

		roomOk := false
		if roomFilterStr != "" {
			_, err := fmt.Sscanf(roomFilterStr, "%d", &roomFilter)
			roomOk = err == nil
		}

		if sessionOk || roomOk {
			var filteredSchedules []model.ExamSchedule
			for _, s := range dist.Schedules {
				keep := true
				if sessionOk && s.SessionNumber != sessionFilter {
					keep = false
				}
				// The frontend model uses an int for RoomID, same as s.RoomID
				if roomOk && s.RoomID != roomFilter {
					keep = false
				}
				if keep {
					filteredSchedules = append(filteredSchedules, s)
				}
			}
			dist.Schedules = filteredSchedules
		}
	}

	if len(dist.Schedules) == 0 {
		c.JSON(http.StatusBadRequest, response.Response{
			Error: &response.ErrorBody{
				Code:    response.ErrValidation,
				Message: "No distribution data available to export",
			},
		})
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	// Default sheet is Sheet1, we can remove it later or rename the first one
	firstSheetRenamed := false

	// Group assignments by schedule
	scheduleMap := make(map[uuid.UUID]model.ExamSchedule)
	for _, s := range dist.Schedules {
		scheduleMap[s.ID] = s
	}

	assignmentsBySchedule := make(map[uuid.UUID][]model.ExamRoomAssignment)
	for _, a := range dist.Assignments {
		assignmentsBySchedule[a.ExamScheduleID] = append(assignmentsBySchedule[a.ExamScheduleID], a)
	}

	// Create styles
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

	for _, s := range dist.Schedules {
		sheetName := fmt.Sprintf("Sesi %d - %s", s.SessionNumber, s.RoomName)
		// Truncate sheet name to 31 chars max (excel limit)
		if len(sheetName) > 31 {
			sheetName = sheetName[:31]
		}

		if !firstSheetRenamed {
			f.SetSheetName("Sheet1", sheetName)
			firstSheetRenamed = true
		} else {
			f.NewSheet(sheetName)
		}

		// Titles
		f.SetCellValue(sheetName, "A1", "DAFTAR HADIR UJIAN")
		f.MergeCell(sheetName, "A1", "F1")
		f.SetCellStyle(sheetName, "A1", "F1", headerStyle)

		f.SetCellValue(sheetName, "A2", fmt.Sprintf("Ujian: %s", exam.Title))
		f.SetCellValue(sheetName, "A3", fmt.Sprintf("Sesi: %d", s.SessionNumber))
		f.SetCellValue(sheetName, "A4", fmt.Sprintf("Ruangan: %s", s.RoomName))

		timeStr := "-"
		if s.StartTime != nil && s.EndTime != nil {
			timeStr = fmt.Sprintf("%s sd %s", s.StartTime.Format("15:04"), s.EndTime.Format("15:04"))
		}
		f.SetCellValue(sheetName, "A5", fmt.Sprintf("Waktu: %s", timeStr))

		dateStr := time.Now().Format("02 January 2006")
		if exam.ScheduledStart != nil {
			dateStr = exam.ScheduledStart.Format("02 January 2006")
		}
		f.SetCellValue(sheetName, "A6", fmt.Sprintf("Tanggal: %s", dateStr))

		// Table headers
		f.SetCellValue(sheetName, "A8", "No")
		f.SetCellValue(sheetName, "B8", "NIS")
		f.SetCellValue(sheetName, "C8", "Nama Siswa")
		f.SetCellValue(sheetName, "D8", "Kelas")
		f.SetCellValue(sheetName, "E8", "Tanda Tangan")
		f.SetCellValue(sheetName, "F8", "")
		f.MergeCell(sheetName, "E8", "F8")

		f.SetCellStyle(sheetName, "A8", "F8", tableHeaderStyle)

		// Set column widths relative to A4
		f.SetColWidth(sheetName, "A", "A", 5)
		f.SetColWidth(sheetName, "B", "B", 15)
		f.SetColWidth(sheetName, "C", "C", 35)
		f.SetColWidth(sheetName, "D", "D", 15)
		f.SetColWidth(sheetName, "E", "E", 12)
		f.SetColWidth(sheetName, "F", "F", 12)

		row := 9
		students := assignmentsBySchedule[s.ID]
		for i, a := range students {
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), i+1)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), a.StudentNIS)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), a.StudentName)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), a.ClassName)

			// Signatures block (odd/even staggered styling)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), "")
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), "")

			if (i+1)%2 == 1 {
				f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("%d.", i+1))
			} else {
				f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), fmt.Sprintf("%d.", i+1))
			}

			// Apply cell styles row by row
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), centerCellStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), centerCellStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("C%d", row), fmt.Sprintf("D%d", row), cellStyle)
			f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row), signatureStyle)

			// Make row taller for signatures
			f.SetRowHeight(sheetName, row, 30)

			row++
		}
	}

	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Error: &response.ErrorBody{
				Code:    response.ErrInternal,
				Message: "Failed to generate Excel file",
			},
		})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=Presensi_Ujian_%s.xlsx", exam.Title))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b.Bytes())
}

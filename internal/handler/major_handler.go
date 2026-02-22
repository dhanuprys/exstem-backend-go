package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

type MajorHandler struct {
	majorService service.MajorService
}

func NewMajorHandler(majorService service.MajorService) *MajorHandler {
	return &MajorHandler{majorService: majorService}
}

type majorRequest struct {
	Code     string `json:"code" binding:"required"`
	LongName string `json:"long_name" binding:"required"`
}

func (h *MajorHandler) GetAll(c *gin.Context) {
	majors, err := h.majorService.GetAllMajors(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"majors": majors})
}

func (h *MajorHandler) Create(c *gin.Context) {
	var req majorRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	major, err := h.majorService.CreateMajor(c.Request.Context(), req.Code, req.LongName)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"major": major})
}

func (h *MajorHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req majorRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	major, err := h.majorService.UpdateMajor(c.Request.Context(), id, req.Code, req.LongName)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"major": major})
}

func (h *MajorHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.majorService.DeleteMajor(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"message": "major deleted successfully"})
}

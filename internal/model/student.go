package model

import "time"

// Gender represents the student's gender.
type Gender string

const (
	GenderMale   Gender = "Laki-laki"
	GenderFemale Gender = "Perempuan"
)

// Religion represents the student's recognized religion.
type Religion string

const (
	ReligionIslam    Religion = "Islam"
	ReligionKristen  Religion = "Kristen"
	ReligionKatolik  Religion = "Katolik"
	ReligionHindu    Religion = "Hindu"
	ReligionBuddha   Religion = "Buddha"
	ReligionKonghucu Religion = "Konghucu"
)

// Student represents a student user.
type Student struct {
	ID           int       `json:"id"`
	NIS          string    `json:"nis"`
	NISN         string    `json:"nisn"`
	Name         string    `json:"name"`
	Gender       Gender    `json:"gender"`
	Religion     Religion  `json:"religion"`
	PasswordHash string    `json:"-"`
	ClassID      int       `json:"class_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// StudentLoginRequest is the payload for student authentication.
type StudentLoginRequest struct {
	NISN     string `json:"nisn" binding:"required,min=4,max=20"`
	Password string `json:"password" binding:"required,min=4,max=128"`
}

// StudentLoginResponse is returned after successful student login.
type StudentLoginResponse struct {
	Token   string  `json:"token"`
	Student Student `json:"student"`
}

// CreateStudentRequest is the payload for creating a new student account.
type CreateStudentRequest struct {
	NIS      string   `json:"nis" binding:"required,min=4,max=20"`
	NISN     string   `json:"nisn" binding:"required,min=4,max=20"`
	Name     string   `json:"name" binding:"required,min=2,max=100"`
	Gender   Gender   `json:"gender" binding:"required,oneof=Laki-laki Perempuan"`
	Religion Religion `json:"religion" binding:"required,oneof=Islam Kristen Katolik Hindu Buddha Konghucu"`
	Password string   `json:"password" binding:"required,min=6,max=128"`
	ClassID  int      `json:"class_id" binding:"required"`
}

// UpdateStudentRequest is the payload for updating an existing student.
type UpdateStudentRequest struct {
	NIS      string   `json:"nis" binding:"required,min=4,max=20"`
	NISN     string   `json:"nisn" binding:"required,min=4,max=20"`
	Name     string   `json:"name" binding:"required,min=2,max=100"`
	Gender   Gender   `json:"gender" binding:"required,oneof=Laki-laki Perempuan"`
	Religion Religion `json:"religion" binding:"required,oneof=Islam Kristen Katolik Hindu Buddha Konghucu"`
	Password string   `json:"password" binding:"omitempty,min=6,max=128"`
	ClassID  int      `json:"class_id" binding:"required"`
}

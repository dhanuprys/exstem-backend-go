package response

// ErrCode is a typed error code enum for consistent API error identification.
type ErrCode string

const (
	// ─── Authentication ────────────────────────────────────────────────
	ErrInvalidCredentials ErrCode = "INVALID_CREDENTIALS"
	ErrSessionActive      ErrCode = "SESSION_ALREADY_ACTIVE"
	ErrSessionInvalidated ErrCode = "SESSION_INVALIDATED"
	ErrTokenRequired      ErrCode = "TOKEN_REQUIRED"
	ErrTokenInvalid       ErrCode = "TOKEN_INVALID"
	ErrTokenExpired       ErrCode = "TOKEN_EXPIRED"

	// ─── Authorization ─────────────────────────────────────────────────
	ErrForbidden         ErrCode = "FORBIDDEN"
	ErrPermissionDenied  ErrCode = "PERMISSION_DENIED"
	ErrStudentAccessOnly ErrCode = "STUDENT_ACCESS_ONLY"
	ErrAdminAccessOnly   ErrCode = "ADMIN_ACCESS_ONLY"

	// ─── Validation ────────────────────────────────────────────────────
	ErrValidation     ErrCode = "VALIDATION_ERROR"
	ErrInvalidID      ErrCode = "INVALID_ID"
	ErrInvalidPayload ErrCode = "INVALID_PAYLOAD"

	// ─── Resources ─────────────────────────────────────────────────────
	ErrNotFound         ErrCode = "NOT_FOUND"
	ErrConflict         ErrCode = "CONFLICT"
	ErrDependencyExists ErrCode = "DEPENDENCY_EXISTS"
	ErrActionForbidden  ErrCode = "ACTION_FORBIDDEN"

	// ─── Exam-specific ─────────────────────────────────────────────────
	ErrExamNotAvailable  ErrCode = "EXAM_NOT_AVAILABLE"
	ErrInvalidEntryToken ErrCode = "INVALID_ENTRY_TOKEN"
	ErrExamNotPublished  ErrCode = "EXAM_NOT_PUBLISHED"
	ErrNotExamAuthor     ErrCode = "NOT_EXAM_AUTHOR"
	ErrNoQuestions       ErrCode = "NO_QUESTIONS"
	ErrExamNotDraft      ErrCode = "EXAM_NOT_DRAFT"
	ErrDuplicateTarget   ErrCode = "DUPLICATE_TARGET_RULE"

	// ─── Media ─────────────────────────────────────────────────────────
	ErrFileRequired    ErrCode = "FILE_REQUIRED"
	ErrUnsupportedFile ErrCode = "UNSUPPORTED_FILE_TYPE"
	ErrFileTooLarge    ErrCode = "FILE_TOO_LARGE"

	// ─── Rate Limiting ─────────────────────────────────────────────────
	ErrRateLimitExceeded ErrCode = "RATE_LIMIT_EXCEEDED"

	// ─── Server ────────────────────────────────────────────────────────
	ErrInternal ErrCode = "INTERNAL_ERROR"
)

// GetMessage returns a human-readable message for a given error code.
func GetMessage(code ErrCode) string {
	switch code {
	// ─── Authentication ────────────────────────────────────────────────
	case ErrInvalidCredentials:
		return "Email/NISN atau kata sandi salah."
	case ErrSessionActive:
		return "Anda sudah login di perangkat lain."
	case ErrSessionInvalidated:
		return "Sesi Anda telah berakhir. Silakan login kembali."
	case ErrTokenRequired:
		return "Token autentikasi diperlukan."
	case ErrTokenInvalid:
		return "Token autentikasi tidak valid."
	case ErrTokenExpired:
		return "Token autentikasi telah kedaluwarsa."

	// ─── Authorization ─────────────────────────────────────────────────
	case ErrForbidden:
		return "Anda tidak memiliki izin untuk mengakses sumber daya ini."
	case ErrPermissionDenied:
		return "Izin ditolak."
	case ErrStudentAccessOnly:
		return "Sumber daya ini terbatas untuk siswa."
	case ErrAdminAccessOnly:
		return "Sumber daya ini terbatas untuk administrator."

	// ─── Validation ────────────────────────────────────────────────────
	case ErrValidation:
		return "Validasi gagal. Silakan periksa masukan Anda."
	case ErrInvalidID:
		return "Format ID tidak valid."
	case ErrInvalidPayload:
		return "Payload permintaan tidak valid."

	// ─── Resources ─────────────────────────────────────────────────────
	case ErrNotFound:
		return "Sumber daya tidak ditemukan."
	case ErrConflict:
		return "Sumber daya sudah ada."
	case ErrDependencyExists:
		return "Data tidak dapat dihapus karena masih digunakan oleh data lain."
	case ErrActionForbidden:
		return "Tindakan ini tidak diperbolehkan."

	// ─── Exam-specific ─────────────────────────────────────────────────
	case ErrExamNotAvailable:
		return "Ujian ini saat ini tidak tersedia."
	case ErrInvalidEntryToken:
		return "Token masuk ujian tidak valid."
	case ErrExamNotPublished:
		return "Ujian ini belum dipublikasikan."
	case ErrNotExamAuthor:
		return "Anda bukan pembuat ujian ini."
	case ErrNoQuestions:
		return "Ujian ini tidak memiliki pertanyaan."
	case ErrExamNotDraft:
		return "Ujian ini tidak dalam status DRAFT."
	case ErrDuplicateTarget:
		return "Aturan target serupa sudah ada untuk ujian ini."

	// ─── Media ─────────────────────────────────────────────────────────
	case ErrFileRequired:
		return "Unggah file diperlukan."
	case ErrUnsupportedFile:
		return "Jenis file tidak didukung."
	case ErrFileTooLarge:
		return "Ukuran file melebihi batas."

	// ─── Rate Limiting ─────────────────────────────────────────────────
	case ErrRateLimitExceeded:
		return "Terlalu banyak permintaan. Silakan coba lagi nanti."

	// ─── Server ────────────────────────────────────────────────────────
	case ErrInternal:
		return "Terjadi kesalahan server internal."
	default:
		return "Terjadi kesalahan yang tidak terduga."
	}
}

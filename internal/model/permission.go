package model

// Permission represents a string code for a specific system action.
type Permission string

const (
	// PermissionMediaUpload allows uploading media files.
	PermissionMediaUpload Permission = "media:upload"

	// PermissionStudentsRead allows viewing student lists and details.
	PermissionStudentsRead Permission = "students:read"

	// PermissionStudentsWrite allows creating and updating students.
	PermissionStudentsWrite Permission = "students:write"

	// PermissionStudentsResetSession allows resetting a student's active session.
	PermissionStudentsResetSession Permission = "students:reset_session"

	// PermissionExamsRead allows viewing exam lists and details.
	PermissionExamsRead Permission = "exams:read"

	// PermissionExamsWriteOwn allows creating exams and updating own exams.
	PermissionExamsWriteOwn Permission = "exams:write_own"

	// PermissionExamsPublish allows publishing exams to make them available to students.
	PermissionExamsPublish Permission = "exams:publish"

	// PermissionAdminsRead allows viewing admin user lists and details.
	PermissionAdminsRead Permission = "admins:read"

	// PermissionAdminsWrite allows creating, updating, and deleting admin users.
	PermissionAdminsWrite Permission = "admins:write"

	// PermissionRolesRead allows viewing admin roles and permissions.
	PermissionRolesRead Permission = "roles:read"

	// PermissionRolesWrite allows creating, updating, and deleting admin roles.
	PermissionRolesWrite Permission = "roles:write"

	// PermissionSettingsRead allows viewing application settings.
	PermissionSettingsRead Permission = "settings:read"

	// PermissionSettingsWrite allows editing application settings.
	PermissionSettingsWrite Permission = "settings:write"

	// PermissionSubjectsRead allows viewing subjects.
	PermissionSubjectsRead Permission = "subjects:read"

	// PermissionSubjectsWrite allows creating, updating, and deleting subjects.
	PermissionSubjectsWrite Permission = "subjects:write"

	// PermissionMajorRead allows viewing majors.
	PermissionMajorRead Permission = "major:read"

	// PermissionMajorWrite allows creating and updating majors.
	PermissionMajorWrite Permission = "major:write"

	// PermissionMajorDelete allows deleting majors.
	PermissionMajorDelete Permission = "major:delete"
)

// AllPermissions is a slice of all available permissions.
var AllPermissions = []Permission{
	PermissionMediaUpload,
	PermissionStudentsRead,
	PermissionStudentsWrite,
	PermissionStudentsResetSession,
	PermissionExamsRead,
	PermissionExamsWriteOwn,
	PermissionExamsPublish,
	PermissionAdminsRead,
	PermissionAdminsWrite,
	PermissionRolesRead,
	PermissionRolesWrite,
	PermissionSettingsRead,
	PermissionSettingsWrite,
	PermissionSubjectsRead,
	PermissionSubjectsWrite,
	PermissionMajorRead,
	PermissionMajorWrite,
	PermissionMajorDelete,
}

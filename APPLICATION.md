High-Traffic MCQ Backend Implementation Plan
Objective: Build a resilient, highly concurrent exam backend capable of handling "Thundering Herd" traffic (700+ students interacting simultaneously) with zero database locking, using a "Grade in RAM, Save in Background" architecture.

Core Tech Stack & Dependencies
Language & Framework: Golang with github.com/gin-gonic/gin

WebSockets: github.com/gorilla/websocket (Integrates perfectly with Gin)

Primary Database: PostgreSQL 15+ using github.com/jackc/pgx/v5/pgxpool for optimal connection pooling.

In-Memory Store & Queue: Redis 7+ using github.com/redis/go-redis/v9

Fast JSON: github.com/goccy/go-json (Compile Gin with -tags=go_json to drastically reduce CPU usage when parsing WebSocket payloads).

Storage: local static

Phase 1: Database Schema (PostgreSQL)
Implement the following normalized tables.

1. School Structure & Users

classes: id, grade_level (INT), major_code (VARCHAR), group_number (INT). (Keep grade and major here for fast querying).

students: id, nisn (UNIQUE), password_hash, class_id (FK to classes).

admins: id, email, password_hash, role_id.

2. RBAC (Role-Based Access Control)

roles: id, name (e.g., "Teacher", "Superadmin").

permissions: id, code (e.g., exams:write_own, students:read).

role_permissions: role_id, permission_id.

3. Exam Engine

exams: id (UUID), title, author_id (FK to admins), scheduled_start, scheduled_end, duration_minutes, entry_token (VARCHAR 6), status.

exam_target_rules: id, exam_id, target_type (CLASS, GRADE, MAJOR), target_value.

questions: id (UUID), exam_id, question_text (HTML), options (JSONB), correct_option, order_num.

exam_sessions: id (UUID), exam_id, student_id, started_at, finished_at, status (IN_PROGRESS, COMPLETED), final_score. (Add a UNIQUE constraint on exam_id, student_id to prevent double-taking).

Phase 2: API Routing Blueprint (Gin Framework)
This is the exact routing map your team should implement, heavily utilizing Gin Route Groups (r.Group()) to apply middlewares cleanly.

A. Public / Authentication Routes (Rate Limited)
POST /api/v1/auth/student/login -> Bcrypt check (cost 8 for speed). Returns Student JWT.

POST /api/v1/auth/admin/login -> Bcrypt check (cost 10+). Returns Admin JWT + permissions array.

B. Student Routes (The "High Traffic" Zone)
Middlewares: RequireStudentJWT(), CheckSingleDeviceSession()

GET /api/v1/student/lobby -> Returns list of exams (Available, In-Progress, Completed) based on the student's class_id.

POST /api/v1/student/exams/:exam_id/join -> Validates the 6-character entry_token. If valid, inserts/updates exam_sessions (sets started_at).

GET /api/v1/student/exams/:exam_id/paper -> Fetches the exam questions (JSONB payload) from Redis (bypassing Postgres). Strips out the correct_option.

C. Student WebSocket (The "Real-Time" Engine)
Middlewares: RequireStudentWSAuth() (Pass JWT via query param ?token=...).

WS /ws/v1/student/exams/:exam_id/stream -> Upgrades to Gorilla WebSocket.

Event: {"action": "autosave", "q_id": "...", "ans": "B"} -> Writes to Redis buffer. Returns {"event": "ack"}.

Event: {"action": "submit"} -> Triggers RAM grading, pushes to Redis scoring queue. Returns {"event": "graded", "score": 85}.

D. Admin & Teacher Routes (The "Management" Zone)
Middlewares: RequireAdminJWT(), RequirePermission("...")

POST /api/v1/admin/media/upload -> Streams upload to S3/Disk, returns {"url": "https://cdn..."}.

GET /api/v1/admin/exams -> Lists exams (filters by author_id if not superadmin).

POST /api/v1/admin/exams -> Creates new draft exam.

POST /api/v1/admin/exams/:exam_id/publish -> CRITICAL: Changes status to PUBLISHED, reads questions from Postgres, and caches the student payload and answer key into Redis.

POST /api/v1/admin/exams/:exam_id/questions -> Adds a question (Handles JSONB options).

Gin Code Structure Example (main.go / router.go)
Go
func SetupRouter(db *pgxpool.Pool, rdb *redis.Client) \*gin.Engine {
router := gin.Default()

    // 1. Auth Group
    auth := router.Group("/api/v1/auth")
    {
        auth.POST("/student/login", handlers.StudentLogin(db, rdb))
        auth.POST("/admin/login", handlers.AdminLogin(db, rdb))
    }

    // 2. Student Group
    studentAPI := router.Group("/api/v1/student")
    studentAPI.Use(middleware.RequireStudentJWT(), middleware.CheckSingleDeviceSession(rdb))
    {
        studentAPI.GET("/lobby", handlers.GetLobby(db))
        studentAPI.GET("/exams/:exam_id/paper", handlers.GetExamPaper(rdb)) // Hits Redis
    }

    // 3. WebSocket Group
    ws := router.Group("/ws/v1")
    ws.Use(middleware.RequireStudentWSAuth())
    {
        ws.GET("/student/exams/:exam_id/stream", handlers.ExamWebSocketStream(rdb))
    }

    // 4. Admin Group
    adminAPI := router.Group("/api/v1/admin")
    adminAPI.Use(middleware.RequireAdminJWT())
    {
        adminAPI.POST("/media/upload", middleware.RequirePermission("exams:write_own"), handlers.UploadMedia())
        adminAPI.POST("/exams/:exam_id/publish", middleware.RequirePermission("exams:write_own"), handlers.PublishExam(db, rdb))
    }

    return router

}
Phase 3: The "Fast Lane" & Background Workers (Data Flow)
The database must be shielded from active exam traffic.

1. The Redis Buffer (WebSocket Autosave)

Student sends answer via WS. Go saves it to a Redis Hash: student:{id}:exam:{id}:answers.

Go instantly sends {"status":"saved"} back to WS.

Go pushes the payload to a Redis List: persist_answers_queue.

2. Background Workers (The "Slow Lane")
   Create separate Goroutines that run infinitely, consuming Redis queues.

Autosave Worker: Pops from persist_answers_queue. Uses SQL UPSERT (ON CONFLICT DO UPDATE) to save answers to Postgres without locking.

Scoring Worker: Pops from persist_scores_queue. Updates exam_sessions with final_score.

Retry Logic: If Postgres times out, the worker must catch the error, Sleep(5s), and push the job back to the Redis queue.

Phase 4: Media & Image Handling
Strict Rule: No Base64 encoded images in the database.

Teacher pastes an image in the frontend Rich Text Editor.

Frontend uploads via POST /api/v1/admin/media/upload.

Go backend saves to AWS S3/Object Storage, returning a public URL.

Frontend embeds <img src="https://cdn.../image.png" />.

Database stores the lightweight HTML string. Serve images through Cloudflare CDN.

Phase 5: Performance & Stability Notes
Connection Pooling: Do not leave SetMaxOpenConns at default. Set PostgreSQL max open connections in pgxpool to roughly (Total CPU Cores \* 4). Too many open DB connections cause context-switching overhead.

Single Device Enforcement: Store login:{student_id} -> {jwt_jti} in Redis. Your CheckSingleDeviceSession middleware checks this key to instantly kick out the first device if they log in elsewhere.

Database Indexing: Ensure you have composite indexes on exam_sessions(student_id, exam_id) and exam_target_rules(exam_id, target_type).

Graceful Shutdown: Implement os.Signal handling for SIGTERM. If you restart the Go server, it must pause accepting new HTTP requests and wait for the Background Workers to finish emptying the Redis queues into PostgreSQL before actually shutting down.

WebSocket Deadlines: Always set SetReadDeadline and SetWriteDeadline on Gorilla WS connections. If a student's internet drops, you do not want dead connections lingering in Go's memory.

Implementation Phasing (Order of Operations)
Data Layer: Postgres schemas and pgxpool / redis connections.

Auth & Middleware: JWT generation and Gin middlewares.

Admin CRUD: Endpoints for teachers to create exams/questions and upload media.

The Core Engine: The /publish endpoint to populate Redis, followed by the Websocket autosave and RAM grading logic.

Workers: Background goroutines that pull from Redis and save to Postgres.

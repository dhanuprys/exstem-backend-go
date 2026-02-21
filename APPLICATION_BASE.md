High-Traffic MCQ Backend Implementation Plan
Objective: Build a resilient, highly concurrent exam backend capable of handling "Thundering Herd" traffic (700+ students interacting simultaneously) with zero database locking, using a "Grade in RAM, Save in Background" architecture.

Core Tech Stack:

Language: Golang (Gin framework recommended)

Primary Database: PostgreSQL 15+

In-Memory Store & Message Queue: Redis 7+

Protocol: HTTP/REST (Auth, Admin) + WebSockets (Active Exam)

Storage: local static

Phase 1: Database Schema (PostgreSQL)
Implement the following normalized tables. Use pgxpool in Golang for optimal connection pooling.

1. School Structure & Users

classes: id, grade_level (INT), major_code (VARCHAR), group_number (INT). (Keep grade and major here for fast querying).

students: id, nisn (UNIQUE), password_hash, class_id (FK to classes).

admins: id, email, password_hash, role_id.

2. RBAC (Role-Based Access Control)

roles: id, name (e.g., "Teacher").

permissions: id, code (e.g., exams:write_own, students:read).

role_permissions: role_id, permission_id.

3. Exam Engine

exams: id (UUID), title, author_id (FK to admins), scheduled_start, scheduled_end, duration_minutes, entry_token (VARCHAR 6), status.

exam_target_rules: id, exam_id, target_type (CLASS, GRADE, MAJOR), target_value.

questions: id (UUID), exam_id, question_text (HTML), options (JSONB), correct_option, order_num.

exam_sessions: id (UUID), exam_id, student_id, started_at, finished_at, status (IN_PROGRESS, COMPLETED), final_score. (Add a UNIQUE constraint on exam_id, student_id).

Phase 2: Authentication & Security Strategy

1. Student Login (High Concurrency)

Bcrypt Tuning: Set bcrypt cost to 8 for students to prevent CPU exhaustion during the 08:00 AM login rush.

Token: Issue a stateless JWT containing student_id and class_id. No database lookups for session validation.

Single Device: Store login:{student_id} -> {jwt_jti} in Redis. Kick old tokens if a new login occurs.

2. Admin Login & RBAC

Inject permissions directly into the Admin JWT payload or store an admin session in Redis.

Middleware: Create an Authorize("exams:write_own") middleware that checks the token, then verifies author_id against the requested exam row.

Phase 3: The "Fast Lane" (Redis Caching & Queues)
The database must be shielded from active exam traffic. The Golang server will interact almost exclusively with Redis during the exam.

1. Pre-Caching (Admin Action)
   When a teacher clicks "Publish Exam", Golang writes this to Redis:

exam:{id}:payload: The full JSON array of questions/options (excluding correct answers).

exam:{id}:key: A hash map of correct answers {"q1": "A", "q2": "C"}.

2. Autosave Buffer (WebSocket)

Student sends: {"q": "q1", "ans": "B"} via WebSocket.

Fast Ack: Go saves to Redis Hash student:{id}:exam:{id}:answers -> Instantly sends {"status":"saved"} back to WS.

Queue: Go pushes the payload to a Redis List: persist_answers_queue.

3. Instant Grading

Student clicks "Submit".

Go compares student:{id}:exam:{id}:answers against exam:{id}:key in RAM.

Go returns the calculated score via HTTP/WS immediately.

Go pushes the final score to persist_scores_queue.

Phase 4: Background Workers (The "Slow Lane")
Create separate Goroutines that run infinitely, consuming Redis queues and safely writing to PostgreSQL.

Autosave Worker: Pops from persist_answers_queue. Uses SQL UPSERT (ON CONFLICT DO UPDATE) to save answers to the database without locking.

Scoring Worker: Pops from persist_scores_queue. Updates the exam_sessions table with the final_score and sets status to COMPLETED.

Retry Logic: If Postgres times out, the worker must catch the error, Sleep(5s), and push the job back to the Redis queue. Zero data loss.

Phase 5: Media & Image Handling
Strict Rule: No Base64 encoded images.

Teacher pastes an image in the frontend Rich Text Editor.

Frontend uploads via POST /api/admin/upload-image.

Go backend saves to AWS S3, DigitalOcean Spaces, or a local public/ directory, returning a public URL.

Frontend embeds <img src="https://cdn.../image.png" />.

Database stores the lightweight HTML string. Serve images through Cloudflare CDN to save server bandwidth.

Phase 6: Additional Performance Improvements (For the Dev Team)
Fast JSON Serialization: The standard library encoding/json in Go uses reflection and is relatively slow. For your WebSocket autosave router, switch to goccy/go-json or bytedance/sonic. This will drastically reduce CPU usage when parsing thousands of JSON payloads per second.

Connection Pooling Config: Do not leave SetMaxOpenConns at default. Set PostgreSQL max open connections in Go to roughly (Total CPU Cores \* 4). Too many open DB connections will cause context-switching overhead.

Database Indexing: Ensure you have composite indexes on exam_sessions(student_id, exam_id) and exam_target_rules(exam_id, target_type).

Graceful Shutdown: Implement os.Signal handling for SIGTERM. If you need to restart the Go server, the application must pause accepting new requests and wait for the Background Workers to finish emptying the Redis queues into PostgreSQL before actually shutting down.

WebSocket Read/Write Deadlines: Always set SetReadDeadline and SetWriteDeadline on WebSocket connections. If a student's internet drops, you do not want dead connections lingering in Go's memory indefinitely.

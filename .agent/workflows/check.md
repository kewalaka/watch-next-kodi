---
description: Perform a full-stack health check (Build, Lint, and Integration)
---

This workflow implements a "Closed-Loop" verification of the entire application. It ensures both backend and frontend are healthy before finishing a task.

# Steps

0. **Verify linting issues first**
   - Make sure all linting issues, and problems reported by the IDE are resolved before progressing

1. **Verify Backend Health**
   - Run `go build -o server ./main.go` to ensure the Go code compiles.
   - Run `go vet ./...` to check for common mistakes.

2. **Verify Frontend Health**
   - Navigate to `web` directory.
   - Run `npm run build` to ensure the React/Vite code compiles without errors.

// turbo
3. **Integration Test (Closed-Loop)**
   - Start the server in the background: `PORT=8091 MOCK_KODI=true ./server &`
   - Store the PID: `SERVER_PID=$!`
   - Wait 2 seconds for boot.
   - Send health check: `curl -v http://localhost:8091/api/health`
   - Verify response contains `"status":"ok"`.
   - Clean up: `kill $SERVER_PID`

4. **Verify Database Integrity**
   - Check if `data/` directory exists and has permissions.
   - Verify SQLite file at `data/whats-next.db` is valid.

5. **Report Result**
   - Provide a final summary of Build, Lint, and Runtime status.
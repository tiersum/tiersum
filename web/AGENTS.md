<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

## Frontend Rules

### No Unit Tests
- Frontend (web/) does not require unit tests
- Do not create `.test.tsx`, `.spec.tsx`, or any test files
- Do not create `__tests__` directories
- Do not create mocks in `__mocks__` directories
- Testing is done manually via the UI

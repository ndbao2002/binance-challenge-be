# AI Usage Report

This project was developed with AI assistance and follows a plan-first workflow.

## 1) Tools Used

Primary toolchain:

- GitHub Copilot Chat Agent
- Model: GPT-5.3-Codex (Medium mode)

Supporting tools during development:

- VS Code integrated terminal
- Docker / Docker Compose
- Go toolchain (go test, go run)
- Python/uv toolchain (uv sync, notebook execution)

## 2) Generated vs. Manually Written

### AI-generated (majority)

- Service scaffolding and iterative implementation for:
  - services/ingestor
  - services/go-backend
  - services/react-frontend
- SDK implementation:
  - sdk/python/binance_sdk
  - sdk/go/binance_sdk
- Database schema and compose wiring:
  - db/init/001_schema.sql
  - docker-compose.yml
- Documentation drafting and rewrites:
  - README.md
  - ingestor runtime documentation

### Manually provided / manually controlled

- Requirement interpretation and acceptance criteria decisions
- Runtime verification and approval of behavior
- Final code review and adjustments for edge cases
- Symbol list and environment decisions in .env.example
- Final review, corrections, and submission decisions

## 3) How To Set Up This Project With AI Tools/Agents

This is the workflow used for this repository:

1. Start each task by asking the agent to read requirements and produce/maintain a concrete plan.
2. Execute implementation in small batches (infra -> ingestor -> backend -> frontend -> SDK -> docs).
3. After each batch, run validations (go test, build checks, docker compose checks).
4. Ask the agent to update docs/evidence continuously (not only at the end).
5. Re-check against REQUIREMENTS.md before finalizing.

Recommended prompt style for reproducibility:

- "Read REQUIREMENTS.md, propose a step-by-step plan, then implement step 1 only."
- "Run tests/build for modified services and fix any errors before moving on."
- "Re-check requirement coverage and list missing items explicitly."

## 4) Prompting Strategy for Complex Logic

Complex logic areas in this project:

- Binance order book synchronization (U/u/pu continuity)
- Reconnect and resync behavior
- Idempotent DB writes across replay/reconnect
- Startup backfill behavior when exact historical depth is unavailable

Prompting pattern used:

1. Constraint-first prompt
   - Define exact exchange rules, invariants, and failure cases.
2. State-machine prompt
   - Ask the agent to model per-symbol lifecycle: seed -> buffer -> sync -> ready -> resync.
3. Adversarial prompt
   - Ask for edge cases explicitly (out-of-order events, continuity breaks, duplicate trades).
4. Verification prompt
   - Require tests/log checks and require fixing detected runtime issues.

**Note**: 
- This pattern was applied iteratively, especially for the ingestor logic, to ensure robust handling of real-world Binance stream behavior.
- You are suggested to manually review the code and documentation for any AI-generated content, especially around complex logic areas, to ensure correctness and completeness before submission.
- Please refer to the `INGESTOR.MD` file for detailed documentation on the ingestor's runtime flow and logic, which was developed with AI assistance but should be manually audited for accuracy.
# Contributing to Valiant

Thanks for your interest in contributing to Valiant! This guide will help you get started with setting up your development environment, running tests, submitting pull requests, and following our code style guidelines.

## Getting Started

### Prerequisites

Before you begin, make sure you have the following installed:
- Go 1.20 or higher
- Node.js 18 or higher
- Docker and Docker Compose
- PostgreSQL 12+ (or use Docker)
- Git

### Setting Up Your Development Environment

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/valiant.git
   cd valiant
   ```

2. **Set up the backend**
   ```bash
   cd backend
   cp ../example/config.yaml config.yaml
   # Edit config.yaml to match your local setup if needed
   go mod download
   ```

3. **Set up the frontend**
   ```bash
   cd ../frontend
   npm install
   ```

4. **Start the development environment**
   ```bash
   # From the project root
   docker-compose up -d db prometheus
   
   # In one terminal, start the backend
   cd backend
   go run cmd/valiant/main.go
   
   # In another terminal, start the frontend
   cd frontend
   npm run dev
   ```

5. **Verify everything works**
   - Backend health check: `curl http://localhost:8080/healthz`
   - Frontend UI: Open `http://localhost:3000` in your browser

## Running Tests

### Backend Tests

```bash
cd backend
go test ./...
```

For tests with coverage:
```bash
go test -cover ./...
```

To run specific tests:
```bash
go test -v ./internal/correlator
```

### Frontend Tests

```bash
cd frontend
npm test
```

For watch mode during development:
```bash
npm test -- --watch
```

## Making Changes

### Creating a Branch

Always create a new branch for your work:
```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/bug-description
```

Use descriptive branch names:
- `feature/add-git-collector` for new features
- `fix/prometheus-query-timeout` for bug fixes
- `docs/improve-installation` for documentation

### Code Style Guidelines

#### Go Backend

- Follow standard Go conventions (use `go fmt` and `go vet`)
- Write clear, descriptive variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and small
- Handle errors explicitly - don't ignore them

Example:
```go
// Good
func fetchChangeEvents(db *sql.DB, serviceID string) ([]ChangeEvent, error) {
    // Implementation
}

// Avoid
func get(d *sql.DB, s string) []ChangeEvent {
    // Implementation that ignores errors
}
```

#### Frontend (React/Next.js)

- Use functional components with hooks
- Follow existing naming conventions
- Use TypeScript types where applicable
- Keep components focused on a single responsibility
- Use meaningful component and variable names

#### General Guidelines

- Write clear commit messages (see below)
- Add tests for new features and bug fixes
- Update documentation if you change functionality
- Keep pull requests focused on a single issue

### Writing Commit Messages

Use clear, descriptive commit messages:

```
feat: add Git collector for repository tags

- Implement Git API client
- Add tag event parsing
- Include unit tests for Git collector

Fixes #123
```

Format: `type: brief description`

Types:
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation changes
- `refactor:` code refactoring
- `test:` adding or updating tests
- `chore:` maintenance tasks

## Submitting Pull Requests

1. **Before submitting**
   - Run all tests locally
   - Ensure your code follows the style guidelines
   - Update documentation if needed
   - Rebase on the latest main branch

2. **Create the pull request**
   - Use a clear, descriptive title
   - Reference related issues (e.g., "Fixes #123")
   - Describe what changes you made and why
   - Include screenshots for UI changes

3. **PR Template**
   ```
   ## Description
   Brief description of what this PR does.

   ## Changes Made
   - Change 1
   - Change 2

   ## Testing Done
   - Test case 1
   - Test case 2

   ## Related Issues
   Fixes #issue_number
   ```

4. **After submitting**
   - Respond to review feedback promptly
   - Make requested changes in new commits
   - Be patient - reviews may take a few days

## What We're Looking For

### Good Pull Requests

- Small, focused changes that address one issue
- Well-tested code with passing tests
- Clear commit history
- Updated documentation
- Explained design decisions

### What to Avoid

- Large "big bang" PRs with multiple unrelated changes
- Untested changes to core logic (correlator engine, Prometheus queries)
- Adding complex dependencies without discussion
- Implementing features from the "Non-Goals" section
- AI-generated code (see below)

## AI-Assisted Coding Policy

The use of AI-assisted tools (e.g. Copilot, ChatGPT, Cursor) is permitted **only as a limited supportive aid**, such as for:
- understanding existing code,
- exploring design alternatives,
- minor refactoring suggestions,
- syntax or API reminders.

**AI tools must not be used to generate substantial portions of code.**

### Explicitly prohibited
- submitting code that is fully or primarily generated by AI,
- pasting AI-generated implementations with minimal human modification,
- generating complete features, modules, services, or commits using AI tools,
- using AI to avoid understanding the code being submitted.

All contributions **must be authored, reviewed, and fully understood by the contributor**.

### AGPL-3.0 compliance
By submitting a contribution, the contributor confirms that:
- the code is original or derived from sources compatible with **AGPL-3.0**,
- no AI-generated content with unclear, restrictive, or incompatible licensing has been introduced,
- the contributor retains the right to license the contribution under **AGPL-3.0**.

The contributor is solely responsible for ensuring that the use of AI tools does **not violate copyright, licensing terms, or the AGPL-3.0 license**.

### Maintainer rights
Maintainers reserve the right to:
- request explanations, design rationale, or step-by-step walkthroughs of any contribution,
- reject or revert any contribution that appears to be AI-generated or insufficiently understood by the contributor,
- reject contributions **without further justification** if AI misuse or licensing risk is suspected.

Failure to clearly explain a contribution is sufficient grounds for rejection.

**In short:** AI may assist your reasoning - **it may not write the code for you**.

## Getting Help

- Check existing [issues](https://github.com/BytePeaks/valiant/issues) and [pull requests](https://github.com/BytePeaks/valiant/pulls)
- Read the [README](README.md) for architecture and design principles
- For questions, open a discussion or comment on a relevant issue
- Be respectful and patient when asking for help

## Code Review Process

1. A maintainer will review your PR within a few days
2. They may request changes or ask questions
3. Make the requested changes and push new commits
4. Once approved, a maintainer will merge your PR
5. Your contribution will be part of the next release!

## License

By contributing to Valiant, you agree that your contributions will be licensed under the [AGPL-3.0 License](LICENSE).

---

Thank you for contributing to Valiant! Your time and effort help make this project better for everyone.
# GoSentry — Review Agenda

What to look at when reviewing the project as a whole, as opposed to a single
diff. This is the agenda; the rules a review checks against live in
[STANDARDS.md](STANDARDS.md) and [ARCHITECTURE.md](ARCHITECTURE.md).

Scope note: a normal pull-request review checks the change. This agenda is for
a periodic sweep of the whole codebase, so a pass may legitimately end with
"nothing to report" on most items.

## 1. Architecture and project structure

Does the code still match the package map and the event flow in
[ARCHITECTURE.md](ARCHITECTURE.md)? Watch for the boundaries that matter here:
`app.Service` as the sole owner of job and runtime state, the UI reading it
through typed events, `domain` staying free of I/O, and platform-specific code
staying behind the `platform/*` interfaces.

## 2. Complexity against the size of the project

GoSentry is a single-process desktop app with two direct dependencies. Flag
abstraction that is not paying for itself: interfaces with one implementation
and no test seam, indirection added for a use case nobody has asked for, a new
dependency where thirty lines of standard library would do. Also check the
opposite direction — files that have grown past the size guideline in
ARCHITECTURE and should be split the way `jobs_view.go` was.

## 3. Code quality

The checkable rules are in [STANDARDS.md](STANDARDS.md) — error handling, unit
tests for pure helpers, regression tests for fixes, `fyne.Do` for updates off
the main thread. Beyond them: concurrency around `Service.mu`, goroutines whose
lifetime is not obvious, and error paths that report something less useful than
what they caught.

## 4. Documentation and comments

Does every documented behavior still exist, and does every non-obvious behavior
get documented? Check the doc set against the code: README (user-facing
behavior and config keys), ARCHITECTURE (packages and flows), STANDARDS
(rules and intentional behavior), DEVELOPMENT (build), TESTS, PERFORMANCE,
CHANGELOG (an entry per notable change). For comments, the bar is *why*, not
*what* — a comment restating the line below it is noise; an unexplained
workaround is a finding.

## 5. Readability and maintainability

Read a package as someone who has not seen it before. Can the next change be
made without reverse-engineering? Naming that matches the domain vocabulary,
functions that do one thing, and control flow that does not need a diagram.

## 6. Logical errors

Correctness independent of style: scheduling and timing edge cases (overlap
policy, sequential mode, pause interactions), off-by-one and boundary handling,
zero values that mean something (see the timeout rules in STANDARDS), state
that can be observed mid-update, and error paths that leave state inconsistent.

## 7. Legacy code and migrations

The app has no database, so migration means file compatibility: `gosentry.json`
and `jobs.json` written by an older version must keep working. Check that new
`Config` fields are backward compatible, that normalization happens in one
place, and that values which are meaningful zeros are not normalized away. Also
look for code kept alive only for a case that no longer exists.

## 8. Undocumented or under-documented contentious decisions

Any decision a future reader would question needs its reasoning recorded where
it lives: a comment at the code, an entry in the "Intentional behavior" section
of [STANDARDS.md](STANDARDS.md), or — when the work is deferred rather than
decided — a note in [ROADMAP.md](ROADMAP.md), which is where the frozen
window-size work keeps its rationale.

## 9. Other improvement proposals

Anything that does not fit above: build and release ergonomics, test coverage
gaps, dependency health, UX rough edges.

## What happens to the findings

- A defect → fix it, with a regression test when severity is medium or higher.
- Behavior that turns out to be deliberate → record it under "Intentional
  behavior" in [STANDARDS.md](STANDARDS.md) so it is not re-reported.
- Work larger than a single fix → [ROADMAP.md](ROADMAP.md), with the reasoning.
- A new rule the review establishes → [STANDARDS.md](STANDARDS.md).

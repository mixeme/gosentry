---
description: Review the project as a whole against the agenda in docs/REVIEW.md
---

Perform a whole-project review of GoSentry.

Read [docs/REVIEW.md](../../docs/REVIEW.md) first — it is the agenda, and its
nine sections are the areas to cover. Read [docs/STANDARDS.md](../../docs/STANDARDS.md)
and [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) for the rules and
contracts the code is checked against.

$ARGUMENTS narrows the review when given — a package path, a file, or the name
of an agenda section. With no arguments, sweep the whole `src/` tree.

Rules for the report:

- Anything listed under "Intentional behavior" in STANDARDS.md is not a finding.
  If you believe such an entry is now wrong, say so explicitly as a challenge to
  the decision rather than reporting it as a bug.
- Verify before reporting. Read the surrounding code and, where cheap, confirm
  the behavior with a test rather than reasoning about it alone.
- Group findings by agenda section, most severe first, each with the file and
  line and what would actually go wrong.
- Report honestly that a section is clean rather than inventing something for it.
- Do not fix anything during the review. Report first; apply fixes only when
  asked, following "What happens to the findings" in REVIEW.md.

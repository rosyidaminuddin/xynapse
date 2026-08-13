---
name: analyze-ticket
description: Analyze a Jira ticket and produce a step-by-step implementation plan
license: MIT
compatibility: opencode
---

# Analyze a Jira ticket into an implementation plan

The user provides a Jira ticket dossier (key, summary, status, assignee, and
plain-text description). Read it carefully and produce a concrete, actionable
implementation plan.

## Output format

Return a markdown document with these sections:

1. **Summary** — one short paragraph restating what the ticket asks for.
2. **Requirements** — a checklist of the functional requirements implied by
   the description. Use `- [ ]` checkboxes.
3. **Design considerations** — anything that constrains the implementation
   (existing patterns, architecture, edge cases, risks). Only if relevant.
4. **Confirmations** — decisions the implementer would otherwise have to ask
   the user about during implementation. Capture every such decision as a
   numbered question with a suggested default, one per line. If nothing needs
   confirming, write `None.`.
5. **Implementation steps** — ordered steps, each with a filename or component
   it touches and a short description of what changes. Be specific and
   concrete; avoid generic advice.
6. **Acceptance criteria** — how to verify each requirement is met. If the
   ticket dossier includes an `Acceptance Criteria:` block, reuse those
   criteria VERBATIM — the plan must fulfill them, not approximate them. Map
   each requirement to the corresponding ticket AC (reusing the ticket's own
   wording) so they can be traced 1:1. Only when the ticket has no
   `Acceptance Criteria:` block, derive ACs from the description yourself.
7. **Out of scope** — anything the description mentions but that is not
   required for this ticket.

The Confirmations section must follow this exact format so the xynapse CLI can
parse it and ask the user for answers before implementation:

```markdown
## Confirmations

1. Which database should the new table live in? (default: PostgreSQL)
2. Should the migration run automatically on deploy? (default: no)
```

If there is nothing to confirm:

```markdown
## Confirmations

None.
```

## Rules

- Base the plan ONLY on the ticket content provided. Do not invent work that
  the ticket does not ask for.
- When the ticket already has acceptance criteria, they are the authoritative
  definition of done. The plan's Acceptance criteria section must carry them
  verbatim; Implementation steps and Requirements must be shaped to satisfy
  them. Do not rewrite, soften, or substitute different criteria.
- If the description is ambiguous, call out the ambiguity explicitly in
  "Design considerations" and propose the most likely interpretation.
- Never pause for user input during implementation. Any decision the
  implementer would otherwise have to ask about must be captured as a numbered
  question in "Confirmations" with a suggested default.
- Keep each implementation step small enough to review and execute in one
  coding session.
- Do not modify any files. You are only producing a plan.

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
4. **Implementation steps** — ordered steps, each with a filename or component
   it touches and a short description of what changes. Be specific and
   concrete; avoid generic advice.
5. **Acceptance criteria** — how to verify each requirement is met. Reuse the
   same checklist IDs from Requirements so they can be traced 1:1.
6. **Out of scope** — anything the description mentions but that is not
   required for this ticket.

## Rules

- Base the plan ONLY on the ticket content provided. Do not invent work that
  the ticket does not ask for.
- If the description is ambiguous, call out the ambiguity explicitly in
  "Design considerations" and propose the most likely interpretation.
- Keep each implementation step small enough to review and execute in one
  coding session.
- Do not modify any files. You are only producing a plan.

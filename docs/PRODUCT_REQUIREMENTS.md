# Product requirements

Status: Approved initial product definition

## Purpose

Project Progress Register replaces paper and basic spreadsheet project logs with a deliberately simple web register. It is not a Kanban, sprint-management, issue-tracking, or agile-planning product in v1.

The user-facing concepts are only Project, Task, Progress Update, Comment, Suggestion, and Admin Assessment.

## Roles and account administration

V1 has application-local accounts and two global roles: Admin and Member. There is no public self-registration.

Routine account administration must be available to the product frontend through backend APIs. Admins create accounts, disable or re-enable accounts, reset passwords through a secure one-time flow, assign roles, and inspect relevant account and audit information.

Authentication requires secure password hashing, opaque cookie-based sessions, CSRF protection, login throttling, and audit events. The final enabled Admin cannot be disabled or demoted. Initial bootstrap is a one-time setup concern, not routine administration.

## Projects

An Admin creates and edits projects, manages current membership, and configures one current site geofence with latitude, longitude, radius in metres, and maximum acceptable reported accuracy. Projects contain a Markdown description and active/inactive state.

A Member can view a project only while they are a current project member. Possession of an identifier or URL never grants access.

## Tasks

Project members create tasks. Members edit only tasks they created; Admins edit any task. Creator identity is immutable and distinct from the editable set of responsible Members.

A task contains a name, goals, description, zero or more responsible Members, optional target date, chronological progress updates, accepted suggestions, current Admin Assessment, and assessment history. Goals and descriptions are stored as Markdown and rendered as sanitized HTML through a simple formatting toolbar.

## Progress updates

“Add Progress Update” is the central action. A project member may add an update to any task in a project they can currently access. Members edit only their own updates; Admins may review all updates. Every edit preserves an immutable revision with editor, edit time, previous content, and new content.

A text-only update may be submitted without location. Every attached image, document, or video requires a current browser upload location, but an inaccurate or outside-geofence result does not reject the file. Only an image captured through the in-browser camera flow can be labelled verified, and only when its location passes server-side accuracy and geofence evaluation. Existing-file images, documents, and videos are always labelled non-verified while retaining their upload geotag. The server receipt time is authoritative.

The task page presents updates chronologically like entries in a paper work diary.

## Attachments and evidence

Camera captures and existing-file uploads are separate browser-reported attachment sources. Existing uploads are always non-verified even though their upload location is required and stored. Extracted embedded metadata, when supported, is stored separately and labelled untrusted.

For every attachment the system records original name, opaque storage identifier, server-detected MIME type, byte size, SHA-256, uploader, server upload time, source, extracted metadata, and trust classification. Files are never placed in an executable or publicly browsable directory.

V1 uses configurable local filesystem storage behind an application boundary, with metadata in PostgreSQL.

## Comments and suggestions

Project members and Admins comment on updates. Only an Admin can accept a comment as an official suggestion. Acceptance preserves the original comment, records accepting Admin and time, creates an audit event, displays a badge, and surfaces the suggestion at task level. A comment and an accepted suggestion remain separate records.

## Admin assessment

Only an Admin sets or changes the current task assessment. Initial verdicts are On Track, Needs Attention, Blocked, and Complete. Each change appends immutable history; Members see the current assessment prominently and Admins can inspect all history.

## Audit

Append-only audit events cover relevant login failures, login/logout, account administration, projects, membership, geofences, tasks, progress revisions, attachments and downloads, comments, suggestion acceptance, and assessments. Events capture actor, action, target, server time, result, and useful request context without secrets.

Every Admin or current project Member who can view a task can also view its complete oldest-first domain timeline: task/progress edits, attachment lifecycle and downloads, comments, accepted suggestions, and assessment history with actor, server time, entity, and event-specific metadata. This user-facing chronology excludes security-only request context such as client IP and request identifiers.

## User experience

The product UI is a separate frontend concern. This repository owns the backend contracts, authorization, state, failure behavior, and recovery semantics needed by a mobile-first accessible client; it does not implement product screens.

Expected frontend screens remain Home, Project, Task, and Add Progress Update. The precise “needs progress update” dashboard rule remains a non-blocking product decision and will not be invented silently by the backend.

## Non-goals for v1

- Kanban boards, sprints, backlogs, story points, or workflow engines.
- Public registration, multi-tenancy, microservices, Redis, containers, or a production Node.js runtime.
- Cryptographic or court-grade proof of camera, time, or location.
- Blockchain or distributed consensus.
- Deployment during the foundation slice.

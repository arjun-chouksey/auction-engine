# Frontend UX contract

## Canonical UI Map

| Capability | Canonical owner | Source of truth | Allowed variants | Verification |
|---|---|---|---|---|
| Select/Listbox | Native HTML select | This contract | role, status, trigger, sort | browser keyboard and popup |
| Date | Native date and datetime-local inputs | `docs/06_FRONTEND_UX.md` | date, local datetime converted to RFC3339 | parsing tests and browser form |
| Form | Shared label/input/error CSS pattern | This contract | login, create, quote | component and browser flow |
| Scrollbar | Global application stylesheet | `DESIGN.md` | stable table gutter | static audit and browser |
| Toast | Inline banner live region | This contract | success, warning, error, extension | component and browser flow |
| CRUD | React route pages and API helper | `docs/04_API_CONTRACTS.md` | create RFQ, submit quote | buyer and supplier browser flow |

- Authentication tokens live only in React memory. A reload requires sign-in again; a 401 returns the user to sign-in through the protected shell.
- Buyers can create RFQs; suppliers can submit quotes only while the canonical RFQ status is `ACTIVE`. Direct access to a role-forbidden route shows a 403 view. The server remains authoritative.
- List status and page live in URL search parameters. Filters reset pagination. Detail bid and event paging remain local to the detail view.
- Initial loads reserve a state panel. Background refresh preserves existing content and reports “Refreshing…”. Errors remain inline with a retry action.
- RFQ creation navigates to the created detail. Accepted quotes stay on detail, announce the result, and refetch canonical state.
- Quote retries reuse the same UUID only while the serialized payload is unchanged. Any intentional field change creates a new key on submission.
- SSE is a signal only. Detail also refetches after connect/reconnect, focus, own mutation, and every 15 seconds.
- Native select/date controls are canonical for this MVP. All timestamps display the browser timezone name, while datetime input is converted to an RFC3339 instant before submission.
- Tables use semantic markup and server pagination. Narrow layouts retain every field through horizontal scrolling.
- Notifications use inline `role=status` or `role=alert`; product flows never use browser alert, confirm, or prompt dialogs.

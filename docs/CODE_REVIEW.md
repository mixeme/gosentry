# GoSentry — Code Review (2026-06-29)

Версия на момент ревью: **0.11.2**

## Итог

| Критерий | Оценка |
|----------|--------|
| Архитектура | 9/10 |
| Сложность vs масштаб | 8/10 |
| Качество кода | 8/10 |
| Поддерживаемость | 8/10 |
| Логические ошибки | 8/10 (после исправлений) |

Проект зрелый и поддерживаемый для десктопного планировщика (~59 `.go`-файлов). Архитектура слоистая, core-логика хорошо протестирована.

## Сильные стороны

- Single-writer `app.Service` с явным locking contract
- Разделение `domain.Job` (durable) и `domain.JobRuntime` (transient)
- Event-driven UI без обратных вызовов в Fyne под lock
- Portable storage от `os.Executable()`
- Инъекция `runJob` и `scheduler.Clock` в тестах
- Подробная документация (`ARCHITECTURE.md`, inline comments)

## Найденные проблемы и статус исправлений

| # | Проблема | Серьёзность | Статус |
|---|----------|-------------|--------|
| 1 | Data race: `store.Paths` в `executeRun` без lock | Высокая | Исправлено |
| 2 | Run стартует при ошибке `SaveJobs` | Средняя | Исправлено |
| 3 | CRUD эмитит events при failed save | Средняя | Исправлено |
| 4 | Overlap queue — только один `Pending` | Средняя | Документировано (by design) |
| 5 | `time.Now()` vs scheduler clock в `startRunLocked` | Низкая | Исправлено |
| 6 | Silent log write failures | Низкая | Исправлено |
| 7 | Невалидный per-job `overlap_policy` | Низкая | Исправлено |
| 8 | Docs drift (YAML, RunNow/pause) | Низкая | Исправлено |

## Намеренное поведение (не баги)

- `RunNow` разрешён при global pause и для disabled jobs
- Sequential mode — FIFO по порядку в `jobs.json`
- Scheduler tick 1s — sub-second `@every` не поддерживается
- Command timeout 30s — глобальный лимит

## Рекомендации на будущее

- UI widget tests или smoke E2E
- Per-job command timeout в конфиге
- Счётчик вместо `Pending bool` для overlap queue (если нужна полная очередь)
- Убрать legacy ticket-ссылки (T3.1) из комментариев

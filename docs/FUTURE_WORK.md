# GoSentry — Future Work

> Временный документ: цель проекта, стандарт качества и открытые задачи.
> После завершения шлифовки переименуем (например, в `STANDARDS.md`).

## Зачем этот документ

**Образцовый проект** — тот, на который можно сослаться как на эталон: архитектура
понятна с первого прочтения, границы пакетов соблюдаются, намеренные компромиссы
задокументированы, поведение воспроизводимо тестами, новый контрибьютор знает
*как* и *почему* писать код здесь.

Оценки зрелости ниже — **зеркало**, не KPI. Поднимать балл ради балла не имеет
смысла; имеет смысл закрывать пункты чеклиста.

## Базовая оценка (внутреннее ревью, 2026-06-29)

| Критерий | Оценка |
|----------|--------|
| Архитектура | 9/10 |
| Сложность vs масштаб | 8/10 |
| Качество кода | 8/10 |
| Поддерживаемость | 8/10 |
| Логические ошибки | 9/10 |

Ревью проводилось на **0.11.2**; исправления вошли в **0.11.3–0.11.4**.
Текущая версия: `src/app/version.go`.

## Архитектурные сильные стороны

- Single-writer `app.Service` с явным locking contract
- Разделение `domain.Job` (durable) и `domain.JobRuntime` (transient)
- Event-driven UI без обратных вызовов в Fyne под lock
- Portable storage от `os.Executable()`
- Инъекция `runJob` и `scheduler.Clock` в тестах
- Подробная документация (`ARCHITECTURE.md`, inline comments)

## Стандарт качества (обязателен для нового кода)

- Контракты пакетов — [ARCHITECTURE.md](ARCHITECTURE.md)
- User-facing error → `dialog.ShowError` или History event, не silent `return`
- Pure helpers → unit-тест в том же пакете
- Фикс severity ≥ medium → regression-тест
- Намеренное поведение → §«Намеренное поведение» ниже, не backlog-баг
- UI view-конструкторы принимают `*app.Service`, не вызывают `app.Open()` внутри

## Чеклист зрелости

| # | Критерий | Статус |
|---|----------|--------|
| 1 | Контракты пакетов задокументированы и соблюдаются | ✓ |
| 2 | Намеренные trade-off'ы явно записаны | ✓ |
| 3 | Нет silent failures в user-facing путях | ✓ |
| 4 | Pure logic вынесена и покрыта unit-тестами | ✓ |
| 5 | Regression-тест на серьёзные фиксы ревью | ✓ |
| 6 | DI на границе UI↔Service | ✓ |
| 7 | Документация = код | ✓ |
| 8 | Platform-код тестируется по одному образцу | ✓ |
| 9 | Единый стиль ошибок в UI | ✓ |

## Намеренное поведение (не баги)

- `RunNow` разрешён при global pause и для disabled jobs
- Sequential mode — FIFO по порядку в `jobs.json`
- Scheduler tick 1s — sub-second `@every` не поддерживается
- Command timeout 30s — глобальный лимит
- **History tab — session-only.** `JobRuntime.Logs` живёт только в памяти
  текущей сессии. Файлы в `logs_dir` используются для aggregate stats
  (`SeedStats`), не для таблицы History. Подробнее — [ARCHITECTURE.md](ARCHITECTURE.md).

## Закрытые находки ревью (2026-06-29)

Историческая справка; не открывать повторно без новых данных.

| # | Проблема | Серьёзность | Статус |
|---|----------|-------------|--------|
| 1 | Data race: `store.Paths` в `executeRun` без lock | Высокая | Исправлено |
| 2 | Run стартует при ошибке `SaveJobs` | Средняя | Исправлено |
| 3 | CRUD эмитит events при failed save | Средняя | Исправлено |
| 4 | Overlap queue — только один `Pending` | Средняя | Исправлено (`PendingRuns`) |
| 5 | `time.Now()` vs scheduler clock в `startRunLocked` | Низкая | Исправлено |
| 6 | Silent log write failures | Низкая | Исправлено |
| 7 | Невалидный per-job `overlap_policy` | Низкая | Исправлено |
| 8 | Docs drift (YAML, RunNow/pause) | Низкая | Исправлено |
| 9 | `StartOnly` игнорировал cancel context | Низкая | Исправлено |
| 10 | `SeedStats` коллизия sanitized имён | Низкая | Исправлено (match по `job_id`) |
| 11 | `AvgDurationMS` seed vs live расходились | Низкая | Исправлено (`TimedRunCount`) |
| 12 | Legacy ticket-ссылки в комментариях | Низкая | Исправлено |

## Вне scope (осознанные trade-off'ы)

| Item | Где зафиксировано |
|------|-------------------|
| History из `.log` | Session-only by design (§выше) |
| Per-job command timeout | [ROADMAP.md](ROADMAP.md) |
| Window size persistence | [ROADMAP.md](ROADMAP.md) (frozen) |
| Column filters в History | [ROADMAP.md](ROADMAP.md) |
| CI coverage gate | [ROADMAP.md](ROADMAP.md) (будущее) |

## Честно про 10/10

Единые **10/10** по всем критериям — неразумная цель для Fyne desktop: CGO,
platform stubs и headless-лимиты GUI объективно добавляют сложность.
**Образцовость ≠ идеальный балл.** Целевое состояние: чеклист зрелости ✓,
проект как reference implementation layered Go desktop app.

## Связанные документы

- [ARCHITECTURE.md](ARCHITECTURE.md) — контракты пакетов
- [TESTS.md](TESTS.md) — как и что тестировать
- [ROADMAP.md](ROADMAP.md) — крупные фичи и platform blockers
- [DEVELOPMENT.md](DEVELOPMENT.md) — сборка и layout

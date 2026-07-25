# План реализации: Per-job command timeout

Реализация пункта **«Per-job command timeout»** из [ROADMAP.md](../ROADMAP.md),
построенная по образцу уже реализованного `overlap_policy` — тот же паттерн
наследования «пусто → глобальный дефолт».

## Рекомендуемая модель

**Claude Opus** (Opus 4.8 или новее).

Обоснование: это не локальная правка, а сквозное изменение через слои
(`domain → storage → runner → app seam → ui → tests`) со сменой сигнатуры
`RunJob` и seam-типа `Service.runJob`, что затрагивает ~9 существующих тестов и
несколько UI-файлов. Нужна аккуратность в резолве эффективного значения под
`mu` и в сохранении контракта «раннер не знает о глобальном конфиге». Для такой
кросс-слойной работы с тестами уместен Opus; Sonnet справится с отдельными
шагами, но выше риск упустить обновление одного из вызовов/тестов.

Замечание по окружению: сборка и тесты GUI требуют **CGO + MSYS2 UCRT64**
(дефолтный Bash-env идёт с `cgo off`).

---

## Модель данных

**[src/domain/job.go](../../src/domain/job.go)** — добавить поле в `Job`:

```go
TimeoutSeconds int `json:"timeout_seconds,omitempty"` // 0 = наследовать глобальный дефолт
```

**[src/domain/config.go](../../src/domain/config.go)** — добавить в `Config`:

```go
DefaultTimeoutSeconds int `json:"default_timeout_seconds,omitempty"`
```

Ключевое соглашение (как у `OverlapPolicy`): `0` на `Job` означает
«наследовать», поэтому `normalizeJob` не должен затирать 0 глобальным значением.

## Дефолты и загрузка

**[src/storage/store.go](../../src/storage/store.go)** — по образцу `OverlapPolicy`:

- в литерал дефолтного `Config` добавить `DefaultTimeoutSeconds: 30`
  (сохраняет текущее поведение — 30 с);
- в блоке нормализации после загрузки:
  `if config.DefaultTimeoutSeconds <= 0 { config.DefaultTimeoutSeconds = 30 }`.

Это переносит нынешнюю константу `commandTimeout = 30s` из `runner.go` в конфиг
как значение по умолчанию.

## Runner (сохранить чистоту контракта)

Раннер не должен знать о глобальном конфиге — эффективный таймаут резолвится в
app-слое и передаётся внутрь.

**[src/runner/runner.go](../../src/runner/runner.go)**:

- сигнатура `RunJob(ctx, job, trigger, logsDir)` →
  `RunJob(ctx, job, trigger, logsDir, timeout time.Duration)`;
- `runCtx, cancel := context.WithTimeout(ctx, timeout)` вместо константы;
- `runStateDetail(...)` принимает `timeout` и печатает его в сообщении
  `"Timed out after %s"` вместо `commandTimeout`;
- **StartOnly не трогаем**: эта ветка использует `jobInvocation(ctx, …)`
  (не `runCtx`), поэтому run-таймаут её не касается — измерение launch latency
  сохраняется. Константу `commandTimeout` можно удалить, `commandWaitDelay`
  оставить.

## App-слой: резолв и проброс

**[src/app/run.go](../../src/app/run.go)** — добавить рядом с
`effectiveOverlapPolicy`:

```go
func (s *Service) effectiveTimeout(job *domain.Job) time.Duration {
    secs := job.TimeoutSeconds
    if secs <= 0 {
        secs = s.store.Config.DefaultTimeoutSeconds
    }
    return time.Duration(secs) * time.Second
}
```

Резолвить под `mu` в `startRunLocked` и класть в снапшот `runEnv` (там же, где
`logsDir`/`maxFiles`) — новое поле `timeout time.Duration`. В `executeRun`
передавать `env.timeout` в `s.runJob(...)`.

**[src/app/service.go](../../src/app/service.go)** — обновить тип seam-поля
`runJob` (добавить `timeout time.Duration`); присваивание
`runJob: runner.RunJob` останется валидным после смены сигнатуры.

## Валидация

**[src/app/operations.go](../../src/app/operations.go)**:

- `validateJob`:
  `if job.TimeoutSeconds < 0 { return errors.New("timeout must be zero (inherit) or a positive number of seconds") }`;
- `validateConfig`:
  `if config.DefaultTimeoutSeconds <= 0 { return errors.New("default timeout must be a positive number of seconds") }`.

## UI

**Диалог задачи [src/ui/job_dialog.go](../../src/ui/job_dialog.go)** — рядом с
«Overlap policy»: числовой `widget.NewEntry` «Timeout (s)» с плейсхолдером-
подсказкой про наследование; пусто → `TimeoutSeconds = 0`, иначе `strconv.Atoi`
с показом ошибки как у schedule.

**Настройки [src/ui/settings_view.go](../../src/ui/settings_view.go)** — в секцию
«Queue» добавить поле «Default timeout (s)» рядом с «Default overlap policy»:
инициализация из `store.Config.DefaultTimeoutSeconds`,
`OnChanged → updateSaveState`, запись в `config.DefaultTimeoutSeconds` при
сохранении и учёт в dirty-check.

**Панель деталей [src/ui/jobs_view_details.go](../../src/ui/jobs_view_details.go)**
— новая строка «Timeout», отображающая эффективное значение через новый хелпер
в [src/app/format.go](../../src/app/format.go):

```go
func DisplayTimeout(job domain.Job, globalDefault int) string // "45 s" или "30 s (global default)"
```

по образцу `DisplayOverlapPolicy`. Прокинуть `globalDefault` в
`newDetailsPanel/update` так же, как уже прокинут `globalOverlapPolicy`.

## Тесты

- **[src/runner/runner_test.go](../../src/runner/runner_test.go)** — все 9 вызовов
  `RunJob` получают новый аргумент; добавить кейс: короткий per-job таймаут →
  `Failed / Timed out after …`; StartOnly с малым таймаутом → не таймаутит.
- **[src/app/run_test.go](../../src/app/run_test.go)** — тест `effectiveTimeout`:
  инхерит при `TimeoutSeconds==0`, собственное значение перекрывает глобальное.
- **[src/app/operations_test.go](../../src/app/operations_test.go)** —
  отрицательный per-job timeout и неположительный default отклоняются.
- **[src/app/format_test.go](../../src/app/format_test.go)** — `DisplayTimeout`
  (собственное значение vs «(global default)»).
- **settings / mainwindow тесты** — при необходимости обновить конструкторы
  `Config`.

## Документация

- Удалить раздел из [ROADMAP.md](../ROADMAP.md).
- Обновить упоминания сигнатуры/таймаута `RunJob` в
  [ARCHITECTURE.md](../ARCHITECTURE.md).
- Запись в [CHANGELOG.md](../CHANGELOG.md).

## Порядок работ

1. `domain` → storage-дефолты (компилируется, поведение прежнее);
2. `runner` + seam-сигнатура + прогон таймаута через `runEnv` → чиним
   компиляцию тестов;
3. валидация;
4. UI (диалог, настройки, детали);
5. тесты + доки.

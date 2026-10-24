# Empyrion Localization Tool

*([English version below](#empyrion-localization-tool-1))*

Инструмент для локализации файлов игры *Empyrion: Galactic Survival*. 
Игровые CSV синхронизируются в единое, версионируемое (через git)
состояние проекта — по ключу и языку. Инструмент позволяет выполнять перевод, 
как одним, так и одновременно несколькими участниками, а также позволяет 
подключать AI для получения чернового варианта и поиска терминов.

Предусмотрена возможность дополнительно переводить
ванильный файл локализации (при переводе сценария), чтобы получить целостный результат.

## Возможности

- **Веб-интерфейс** — как основная рабочая область, имеет систему фильтров для удобства,
  а также рендер текста (как он будет выглядеть в игре).
- **AI-перевод** — любой OpenAI-совместимый провайдер (включая локальный
  Ollama): полный последовательный перевод, либо перевод по ключам (как приоритетный),
  автоматическое обнаружение терминов для глоссария.
- **Глоссарий терминов** — собвственно глоссарий, позволяет поддерживать единообразие, 
  а также передавать термины в AI-переводчик по необходимости. Имеется возможность
  импортировать глоссарий, если он соответствует шаблону.
- **Цепочки диалогов** (Dialogues/PDA) — при подключении соответствующего файла 
  позволяет связывать ключи в единую цепочку, что позволяет отслеживать весь квест/реплики
  полностью и повышать качество и связность перевода.
- **Параллельный перевод без конфликтов** — разбиение проекта на пакеты
  ключей, которые можно раздать нескольким переводчикам одновременно.
  **Блокировка в реальном времени** — выполняется автоматически, блокирует 
  ключ/термин для других участников, если кто-то начал редактирование.
- **История на git** — позволяет просматривать все изменения с возможностью отката к предыдущим версиям.
- **Мультипользовательский режим** — позволяет подключать к переводу несколько человек,
  режим недоступен, пока приложение работает на локальном адресе.
- **Актуализация** — позволяет загружать/обновлять основные файлы без потери прогресса,
  а также импортировать готовый перевод.
  **Экспорт** — позволяет получать итоговый вариант файлов для последующего внедрения в игру.
- **Валидация** — на низком уровне перепроверяет результат перевода через AI,
  на уровне GUI - ведет посчет тегов, спец.последовательностей, плейсхолдеров.

## Быстрый старт

Готовые сборки под Windows/Linux — на странице
[Releases](https://github.com/maximalmax90/EmpyrionLocalizationTool/releases).

## Режимы запуска

```
elt                                    — то же самое, что elt gui (без флагов)
elt gui [--data-dir <dir>]             — локальный/одиночный режим: сервер + браузер
elt gui --addr host:port               — подключиться к уже запущенному elt serve
elt serve --data-dir <dir> --listen host:port [--no-ui]
                                       — только сервер (без --no-ui — с веб-интерфейсом)
elt project <команда> [флаги]          — Полнофункциональный CLI: перевод/синхронизация/AI/глоссарий/история и т.п.
elt decompile <csv-файл>               — разбор структуры CSV без проекта
elt validate --type=... <en> <перевод> — проверка одной строки перевода без проекта
```

У каждой команды (и у каждой из ~50 подкоманд `elt project`) есть
подробная справка: `elt <команда> --help`.

## Подключение к удалённому серверу (для пользователей Windows)

Если проект уже поднят кем-то другим на удаленном хосте и вам нужно просто
подключиться и переводить — свой проект создавать не нужно, приложение
достаточно запустить в режиме тонкого клиента:

1. Узнайте у администратора проекта адрес сервера (`host:port`, либо
   `https://host:port`, если сервер использует HTTPS) и, если сервер
   многопользовательский — свой персональный токен доступа.
2. Откройте командную строку, перейдите в папку с приложением
3. Запустите (замените адрес/порт на реальные):

   ```
   elt-windows-amd64.exe gui --addr 192.168.1.10:8080
   ```

   Откроется браузер с рабочим пространством, подключённым к удалённому
   серверу.
4. Если сервер запросит токен — введите тот, что получили от
   администратора, прямо в открывшейся странице; он запомнится в браузере,
   повторно вводить в будущем не придется.
5. Если сервер использует самоподписанный сертификат HTTPS, добавьте `--insecure`:

   ```
   elt-windows-amd64.exe gui --addr https://192.168.1.10:8080 --insecure
   ```

Чтобы не набирать команду каждый раз, создайте рядом с `exe` файл
`connect.bat` с этой же строкой внутри и просто запускайте его.

## Сборка из исходников

Требуется Go 1.26+ и Node.js (для сборки веб-интерфейса).

```bash
cd web && npm install && npm run build   # собирает веб-интерфейс в internal/webui/dist/
cd ..  && go build -o elt ./cmd/elt      # embed уже собранного dist/ в бинарник
```

Проверка перед коммитом/сборкой релиза:

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

Сборка под другую платформу (кросс-компиляция, CGO не требуется):

```bash
GOOS=windows GOARCH=amd64 go build -o elt-windows-amd64.exe ./cmd/elt
GOOS=linux   GOARCH=amd64 go build -o elt-linux-amd64       ./cmd/elt
```

---

# Empyrion Localization Tool

*([Русская версия выше](#empyrion-localization-tool))*

A localization tool for *Empyrion: Galactic Survival*. The game's CSV
files are synced into a single, git-versioned project state — per key,
per language. It supports translation by one person or several at once,
and can hook up an AI provider for a draft translation and term
discovery.

It can also optionally track the vanilla game's own localization file
(when translating a scenario/mod), so the end result stays consistent
with the base game.

## Features

- **Web interface** — the main workspace, with filters for convenience
  and a live render preview (what the text will actually look like in
  the game).
- **AI translation** — any OpenAI-compatible provider (including a local
  Ollama): a full sequential pass, or a targeted, priority translation
  of specific keys, plus automatic glossary term discovery.
- **Glossary** — keeps terminology consistent, and feeds approved terms
  into the AI translator as needed. Terms can be bulk-imported from a
  file matching the expected format.
- **Dialogue chains** (Dialogues/PDA) — once the matching source file is
  attached, links keys into a single chain, letting you follow an entire
  quest/conversation and keep the translation coherent across it.
- **Conflict-free parallel translation** — split the project into
  batches of keys that several translators can work on at the same time.
  **Real-time locking** — automatic: a key/term being edited by someone
  is locked for everyone else.
- **Git-backed history** — review every change, with the ability to roll
  back to any earlier version.
- **Multi-user mode** — lets several people work on the project together
  (unavailable while the app is running on a purely local address).
- **Sync** — pull in/update the source files without losing existing
  progress, and import an already-translated CSV.
  **Export** — produces the final files ready to drop into the game.
  **Validation** — checked at a low level against the source text (via
  AI as well), and at the GUI level with a live count of tags, escape
  sequences, and placeholders.

## Quick start

Pre-built Windows/Linux binaries are on the
[Releases](https://github.com/maximalmax90/EmpyrionLocalizationTool/releases) page.

## Run modes

```
elt                                    — same as elt gui (no flags)
elt gui [--data-dir <dir>]             — local/solo mode: server + browser
elt gui --addr host:port               — connect to an already-running elt serve
elt serve --data-dir <dir> --listen host:port [--no-ui]
                                       — server only (add nothing, or drop --no-ui, for the web UI too)
elt project <command> [flags]          — full CLI: translate/sync/AI/glossary/history, etc.
elt decompile <csv-file>               — inspect a CSV's structure, no project needed
elt validate --type=... <en> <translation> — check one translated string, no project needed
```

Every command (and every one of `elt project`'s ~50 subcommands) has its
own detailed help: `elt <command> --help`.

## Connecting to a remote server (Windows)

If someone else already has the project running (`elt serve`) and you
just need to connect and translate — you don't create your own project,
just run the app as a thin client:

1. Ask the project's admin for the server address (`host:port`, or
   `https://host:port` if the server uses HTTPS) and, if the server has
   multiple users configured, your own personal access token.
2. Open a command prompt and change into the folder with the app.
3. Run it (replace the address/port with the real one):

   ```
   elt-windows-amd64.exe gui --addr 192.168.1.10:8080
   ```

   A browser will open with the workspace, connected to the remote
   server.
4. If the server asks for a token, enter the one you got from the
   admin right on that page — the browser will remember it, so you won't
   need to type it again later.
5. If the server uses a self-signed HTTPS certificate, add `--insecure`:

   ```
   elt-windows-amd64.exe gui --addr https://192.168.1.10:8080 --insecure
   ```

To avoid typing the command every time, put it in a `connect.bat` file
next to the `.exe` and just run that instead.

## Building from source

Requires Go 1.26+ and Node.js (to build the web interface).

```bash
cd web && npm install && npm run build   # builds the web UI into internal/webui/dist/
cd ..  && go build -o elt ./cmd/elt      # embeds the already-built dist/ into the binary
```

Checks to run before committing/cutting a release:

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

Cross-compiling for another platform (no CGO needed):

```bash
GOOS=windows GOARCH=amd64 go build -o elt-windows-amd64.exe ./cmd/elt
GOOS=linux   GOARCH=amd64 go build -o elt-linux-amd64       ./cmd/elt
```

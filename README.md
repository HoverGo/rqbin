# rqbin

Простой Request Bin: создаёшь корзину, получаешь URL, кидаешь на него вебхуки и смотришь, что пришло.

Стек: Go 1.25+ (`net/http`), PostgreSQL, HTML-шаблоны, чуть HTMX для автообновления списка. Всё поднимается через Docker Compose.

## Что умеет

- `POST /bins` - создать bin
- `GET /b/{id}` - страница с последними запросами
- любой другой метод на `/b/{id}` - сохранить входящий запрос (метод, path, query, headers, body, время)
- список на странице обновляется каждые 2 секунды

## Быстрый старт

Нужны Docker и Docker Compose.

```bash
docker compose up --build
```

Открой [http://localhost:8080](http://localhost:8080), нажми «Создать bin», скопируй webhook URL.

Проверка:

```bash
curl -X POST http://localhost:8080/b/<id> \
  -H "Content-Type: application/json" \
  -d "{\"hello\":\"world\"}"
```

Обнови страницу bin (или подожди пару секунд) - запрос появится в списке.

Остановка:

```bash
docker compose down
```

Данные Postgres остаются в volume `pgdata`. Чтобы стереть всё:

```bash
docker compose down -v
```

## Локальный запуск без Docker (приложение)

База всё равно нужна. Можно поднять только Postgres:

```bash
docker compose up -d db
```

Потом:

```bash
go run ./cmd/rqbin
```

Переменные окружения (или значения по умолчанию):

| Переменная | По умолчанию | Зачем |
|---|---|---|
| `ADDR` | `:8080` | адрес HTTP-сервера |
| `DATABASE_URL` | нет | строка подключения Postgres, обязательна |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | URL, который показываем на странице bin |
| `REQUEST_BODY_LIMIT` | `1048576` | лимит тела запроса в байтах (1 MiB) |

Пример:

```bash
set DATABASE_URL=postgres://rqbin:rqbin@localhost:5432/rqbin?sslmode=disable
go run ./cmd/rqbin
```

На Linux/macOS вместо `set` используй `export`.

Миграции применяются при старте приложения из папки `migrations/`.

## Структура

```
cmd/rqbin/          точка входа
internal/config/    конфиг из env
internal/store/     работа с Postgres
internal/handler/   HTTP и страницы
migrations/         SQL-схема
web/templates/      HTML
web/static/         CSS
```

## Заметки

- `GET /b/{id}` занят интерфейсом. Для проверки вебхука используй `POST`, `PUT`, `PATCH`, `DELETE` и т.п.
- Храним последние запросы на странице (до 50). Это MVP, без авторизации и TTL.
- Пароли в `docker-compose.yml` учебные. Для чего-то публичного поменяй их и не свети сервис наружу без нужды.

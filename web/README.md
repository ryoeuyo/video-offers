# OfferBox — frontend

React + TypeScript + Vite. Тёмная тема, mobile-friendly.

## Запуск

```bash
npm install
npm run dev
```

Dev-сервер на `:5173` проксирует `/api` и `/healthz` на бэкенд `:8080`.
Запустите API отдельно: `make run` из корня репозитория.

## Страницы

| Путь | Описание |
|---|---|
| `/` | Список стримеров, поиск |
| `/s/:username` | Публичная страница + форма предложения |
| `/login`, `/register` | Аутентификация |
| `/sent` | Отправленные офферы |
| `/queue` | Очередь стримера |
| `/settings` | Профиль, роль, приём офферов |

## Сборка

```bash
npm run build
npm run preview
```

Для production убедитесь, что `CORS_ORIGINS` на бэкенде включает origin фронтенда.

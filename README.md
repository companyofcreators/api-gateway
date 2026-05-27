# API Gateway

Единая точка входа для всей платформы. Все клиентские запросы проходят через Gateway.

## Обязанности

- Reverse proxy на внутренние сервисы
- JWT RS256 валидация (публичный ключ, без запросов к auth-service)
- Cookie-based аутентификация (HttpOnly, Secure, SameSite=Lax)
- RBAC middleware (user/master/moderator/admin)
- Rate limiting (Redis sliding window, 100 req/min)
- Request ID, логирование, panic recovery
- Аггрегация данных (user profile + orders)

## Эндпоинты

| Метод | Путь | Назначение | Auth |
|-------|------|-----------|------|
| `GET` | `/health` | Health check | Нет |
| `GET` | `/api/v1/profile` | Агрегированный профиль | Да |
| `*` | `/api/v1/auth/*` | Прокси на auth-service (:8081) | Нет |
| `*` | `/api/v1/users/*` | Прокси на user-service (:8082) | Да |
| `*` | `/api/v1/orders/*` | Прокси на order-service (:8083) | Да |
| `*` | `/api/v1/offers/*` | Прокси на offer-service (:8084) | Да |
| `*` | `/api/v1/chat/*` | Прокси на chat-service (:8085) | Да |
| `*` | `/api/v1/files/*` | Прокси на file-service (:8086) | Да |
| `*` | `/api/v1/notifications/*` | Прокси на notification-service (:8087) | Да |
| `DELETE` | `/api/v1/admin/users/{id}` | Удаление пользователя | admin |
| `PUT` | `/api/v1/admin/complaints/{id}` | Управление жалобами | moderator |

## Middleware Pipeline

```
RequestID → RealIP → Recovery → Logging → Timeout(30s) → RateLimit → Auth → RBAC → Proxy
```

## Конфигурация

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `ENV` | `dev` | Окружение (dev/prod). В prod куки Secure. |
| `HTTP_ADDRESS` | `:8080` | Адрес HTTP сервера |
| `JWT_PUBLIC_KEY_PATH` | `./keys/public.pem` | Путь к RSA публичному ключу |
| `AUTH_SERVICE_URL` | `http://localhost:8081` | URL auth-service |
| `USER_SERVICE_URL` | `http://localhost:8082` | URL user-service |
| `ORDER_SERVICE_URL` | `http://localhost:8083` | URL order-service |
| `OFFER_SERVICE_URL` | `http://localhost:8084` | URL offer-service |
| `CHAT_SERVICE_URL` | `http://localhost:8085` | URL chat-service |
| `FILE_SERVICE_URL` | `http://localhost:8086` | URL file-service |
| `NOTIFICATION_SERVICE_URL` | `http://localhost:8087` | URL notification-service |
| `REDIS_ADDRESS` | `localhost:6379` | Адрес Redis |
| `LOG_LEVEL` | `info` | Уровень логирования |

## Безопасность

- JWT в HttpOnly cookie `access_token` (не в localStorage)
- Authorization header stripping перед проксированием
- Внутренние сервисы получают X-User-Id, X-User-Email, X-User-Role
- RS256: gateway имеет только публичный ключ
- Rate limiting: 100 req/min per IP, 5 req/min на login/register

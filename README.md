# Telegram VPN Deploy Bot (OLCRTC)

Серверный Telegram-бот на Go для автоматизированного запуска и интерактивного прохождения инсталлятора `olcrtc` в псевдотерминале (PTY) с извлечением VPN-конфига и отправкой пользователю.

## 🚀 Возможности

- **Интерактивное прохождение в PTY**: Запуск `bash -c "curl -fsSL ... | bash"` внутри реального псевдотерминала (`github.com/creack/pty`), передача последовательности ответов на интерактивные запросы скрипта.
- **Парсинг VPN URI**: Извлечение конфигурационной ссылки (`uri: vless://...`, `trojan://...`, `vmess://...`, `ss://...`) и отправка в Telegram в виде моноширинного блока для удобного копирования.
- **Строгая безопасность**:
  - **Allowlist по User ID**: Команду `/deploy` могут вызывать только заранее одобренные числовые Telegram ID (не юзернеймы).
  - **Фиксированный URL скрипта**: Адрес инсталлятора жёстко задан в конфигурации и не может быть изменён через Telegram.
  - **Single-Flight Lock**: Гарантия выполнения только одной установки одновременно на сервере.
  - **Rate Limiting (Cooldown)**: Ограничение частоты повторных запусков на пользователя (по умолчанию 10 минут).
  - **Таймаут выполнения**: Принудительное завершение зависшего процесса (`SIGKILL`) по истечении таймаута (по умолчанию 5 минут).
  - **Маскирование секретов**: Токены и VPN URI автоматически маскируются в структурированных логах (`vless://[REDACTED]`).
  - **Аудит**: Структурированные записи безопасности для каждого вызова с фиксацией ID пользователя, действия, статуса и длительности.

---

## 📁 Структура проекта

```text
telegram-deploy-bot/
├── cmd/
│   └── bot/
│       └── main.go              # Точка входа, DI, graceful shutdown
├── internal/
│   ├── config/                  # Загрузка и валидация конфига (YAML + ENV)
│   ├── logging/                 # Structured slog, маскирование URI/токенов, аудит
│   ├── bot/                     # Telegram long-polling, хендлеры /start и /deploy
│   └── installer/               # PTY runner, 7-шаговая последовательность, парсер URI
├── configs/
│   └── config.yaml              # Несекретная конфигурация
├── .env.example                 # Шаблон переменных окружения
├── Makefile                     # Команды сборки и тестирования
├── go.mod
└── go.sum
```

---

## ⚙️ Установка и запуск

### 1. Предварительные требования

- Go 1.23+
- Сервер Linux (Ubuntu 22.04+ / Debian 12+) с установленными `bash` и `curl`
- Токен бота от [@BotFather](https://t.me/BotFather)
- Ваш Telegram User ID (можно узнать у [@userinfobot](https://t.me/userinfobot))

### 2. Настройка окружения

```bash
# Клонируйте репозиторий
git clone <repo-url>
cd free-bot

# Скопируйте шаблон секретов
cp .env.example .env

# Укажите токен бота в .env
echo "TG_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ" > .env
```

### 3. Настройка `configs/config.yaml`

```yaml
telegram:
  allowed_user_ids:
    - 123456789 # Укажите ваш числовой Telegram ID

installer:
  script_url: "https://raw.githubusercontent.com/openlibrecommunity/olcrtc/master/install.sh"
  timeout: 5m
  comment_pool: [1, 4, 5, 6, 7, 9, 10, 11, 12, 14, 15]
  comment_suffix: "welcome"

limits:
  cooldown_per_user: 10m

logging:
  level: "info"
  json: true
```

### 4. Запуск через Docker (рекомендуется для серверов)

```bash
# Сборка и фоновый запуск
docker compose up -d --build

# Просмотр логов бота
docker compose logs -f

# Остановка
docker compose down
```

> **Важно**: Контейнер запускается с `network_mode: host` и `privileged: true`, так как инсталлятор `olcrtc` разворачивает VPN-сервер внутри собственного Podman-контейнера и требует прямого доступа к сетевым интерфейсам хоста.

### 5. Локальный запуск (Нативная установка)

Вы можете запустить бота прямо на сервере без Docker (например, если возникают проблемы с cgroups внутри контейнера) при помощи предоставляемого `systemd` шаблона.

```bash
# 1. Сборка бинарника
make build

# 2. Скопируйте бинарник в /usr/local/bin или рабочую директорию бота
sudo cp bot /usr/local/bin/telegram-vpn-deploy-bot

# 3. Скопируйте и настройте service файл
sudo cp scripts/telegram-vpn-bot.service /etc/systemd/system/
# Отредактируйте ExecStart и WorkingDirectory внутри /etc/systemd/system/telegram-vpn-bot.service

# 4. Активируйте и запустите службу
sudo systemctl daemon-reload
sudo systemctl enable --now telegram-vpn-bot
sudo systemctl status telegram-vpn-bot
```

> Не забудьте указать `app.execution_mode: "systemd"` в `config.yaml` для соответствующего логирования.

---

## 🧪 Тестирование

Запуск полного набора юнит-тестов с детектором гонок:

```bash
make test
```

Линтинг и проверка кода:

```bash
make vet
make lint
```

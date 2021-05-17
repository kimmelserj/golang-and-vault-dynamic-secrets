# golang-and-vault-dynamic-secrets

Приложение-пример для демонстрации работы с HashiCorp Vault Database Dynamic Secrets из Golang-кода.

## Быстрый старт

1) Склонировать репу `git clone https://github.com/kimmelserj/golang-and-vault-dynamic-secrets.git`.
2) Перейти в склонированную директорию `cd golang-and-vault-dynamic-secrets`.
3) Запустить `docker-compose down -v && docker-compose up`.
4) Запустить приложение через `VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=app-dev-token go run cmd/golang-and-vault-dynamic-secrets/main.go`. Этот шаг можно выполнить несколько раз, чтобы увидеть, как для каждого процесса создаётся свой пользователь.

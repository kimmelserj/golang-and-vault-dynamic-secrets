package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	dbConnString string
	dbSecretPath string
)

func init() {
	flag.StringVar(&dbConnString, "db-conn-string", "postgres://127.0.0.1:5432/app-db", "Database connect string")
	flag.StringVar(&dbSecretPath, "db-secret-path", "database/creds/app", "Database secret path in HashiCorp Vault")
}

/*
	Приложение периодически отображает список пользователей postgres с указанием даты истечения их срока жизни.
*/
func main() {
	flag.Parse()

	ctx := context.Background()

	vaultClient, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	dbSecret, err := vaultClient.Logical().Read(dbSecretPath)
	if err != nil {
		panic(err)
	}
	defer func() {
		/*
			Отзыв секрета не является обязательной операцией, но лучше подчищать за собой.
			Если приложение будет убито по SIGKILL, то Vault сам отзовёт секрет, когда истечёт TTL у секрета.
		*/
		_ = vaultClient.Sys().Revoke(dbSecret.LeaseID)
	}()

	/*
		Renewer периодически продлевает аренду динамического секрета, чтобы созданный пользователь не был удалён из базы по TTL.
		Периодичность операции продления вычисляется с использованием jitter.
	*/
	dbSecretRenewer, err := vaultClient.NewRenewer(&api.RenewerInput{
		Secret: dbSecret,
	})
	if err != nil {
		panic(err)
	}

	go dbSecretRenewer.Renew()
	defer dbSecretRenewer.Stop()
	go func() {
		for {
			select {
			case err := <-dbSecretRenewer.DoneCh():
				if err != nil {
					/*
						Если данный if выполняется, то это значит что что-то пошло не так при продлении аренды
						и лучше что-то предпринять, так как Renewer завершил свою работу с ошибкой.
						Здесь обязательно нужно дать понять приложению о том что не удалось продлить аренду.
						Иначе приложение будет сыпать ошибками аутентификации при подключении к базе, пока его не перезапустят.
					*/
					panic(err)
				}

			case renewal := <-dbSecretRenewer.RenewCh():
				log.Printf("Database secret has been successfully renewed at %s\n", renewal.RenewedAt.Format(time.RFC3339))
			}
		}
	}()

	dbConfig, err := pgxpool.ParseConfig(dbConnString)
	if err != nil {
		panic(err)
	}

	dbConfig.ConnConfig.User = dbSecret.Data["username"].(string)
	dbConfig.ConnConfig.Password = dbSecret.Data["password"].(string)

	db, err := pgxpool.ConnectConfig(ctx, dbConfig)
	if err != nil {
		panic(err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		err = logPostgresUsers(ctx, db)
		if err != nil {
			panic(err)
		}

		select {
		case <-ticker.C:
		case <-sigChan:
			return
		}
	}
}

func logPostgresUsers(parentCtx context.Context, db *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel()

	const query = `SELECT usename, valuntil FROM pg_user`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			username   string
			validUntil sql.NullTime
		)

		err = rows.Scan(&username, &validUntil)
		if err != nil {
			return err
		}

		invalidationInfo := "never"
		if validUntil.Valid {
			invalidationInfo = validUntil.Time.Format(time.RFC3339)
			invalidationInfo += "\t" + time.Until(validUntil.Time).String()
		}

		log.Printf("User %s will expire at %s\n", username, invalidationInfo)
	}

	return nil
}

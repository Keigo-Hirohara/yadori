package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL が設定されていません")
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	m, err := migrate.New("file://db/migrations", dsn)
	if err != nil {
		return err
	}
	defer m.Close()

	switch command {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "version":
		version, dirty, verr := m.Version()
		if verr != nil {
			return verr
		}
		fmt.Printf("版: %d, 中断された適用: %t\n", version, dirty)
		return nil
	default:
		return fmt.Errorf("知らないコマンドです: %s", command)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("適用するものはありません")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s を完了しました\n", command)
	return nil
}

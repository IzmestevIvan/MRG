// Команда server запускает веб-приложение МРГ.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/izmestevivan/mrg/internal/service"
	"github.com/izmestevivan/mrg/internal/store"
	"github.com/izmestevivan/mrg/internal/web"
)

func main() {
	addr := os.Getenv("MRG_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	st := store.NewMemoryStore()
	svc := service.New(st, nil)
	srv, err := web.NewServer(svc)
	if err != nil {
		log.Fatalf("инициализация шаблонов: %v", err)
	}

	log.Printf("МРГ запущен на http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatalf("сервер остановлен: %v", err)
	}
}

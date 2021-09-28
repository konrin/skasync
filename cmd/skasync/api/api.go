package api

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

func NewAPIListenerAndStart(cfg Config, fn func(*echo.Echo) error) error {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	if err := fn(e); err != nil {
		return err
	}

	return e.Start(fmt.Sprintf("127.0.0.1:%d", cfg.Port))
}
